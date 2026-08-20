package framing

import (
	"encoding/binary"
	"errors"
	"io"
)

// Op is a frame opcode.
type Op byte

const (
	OpText   Op = 0x1
	OpBinary Op = 0x2
	OpClose  Op = 0x8
	OpPing   Op = 0x9
	OpPong   Op = 0xA
)

// ErrBadFrame is returned when the frame header is corrupt.
var ErrBadFrame = errors.New("bad frame")

// ErrFrameTooLarge is returned when the payload exceeds the
// configured limit.
var ErrFrameTooLarge = errors.New("frame too large")

// Frame is one decoded message.
type Frame struct {
	Op      Op
	Payload []byte
}

// Conn wraps an io.ReadWriter with framing.
type Conn struct {
	rw       io.ReadWriter
	maxBytes int
}

// New creates a framed Conn. maxBytes <= 0 means no limit.
func New(rw io.ReadWriter, maxBytes int) *Conn {
	return &Conn{rw: rw, maxBytes: maxBytes}
}

// Write sends a frame with the given opcode and payload.
func (c *Conn) Write(op Op, payload []byte) error {
	size := len(payload)
	var hdr []byte
	switch {
	case size < 126:
		hdr = make([]byte, 2)
	case size < 65536:
		hdr = make([]byte, 4)
	default:
		hdr = make([]byte, 10)
	}
	hdr[0] = 0x80 | byte(op)
	switch {
	case size < 126:
		hdr[1] = byte(size)
	case size < 65536:
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(size))
	default:
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(size))
	}
	if _, err := c.rw.Write(hdr); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := c.rw.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// Read returns the next frame.
func (c *Conn) Read() (*Frame, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(c.rw, hdr[:]); err != nil {
		return nil, err
	}
	fin := hdr[0]&0x80 != 0
	op := Op(hdr[0] & 0x0F)
	masked := hdr[1]&0x80 != 0
	sizeField := hdr[1] & 0x7F
	size := int(sizeField)
	switch sizeField {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
			return nil, err
		}
		size = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
			return nil, err
		}
		size = int(binary.BigEndian.Uint64(ext[:]))
	}
	if c.maxBytes > 0 && size > c.maxBytes {
		return nil, ErrFrameTooLarge
	}
	if !fin {
		return nil, ErrBadFrame
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.rw, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, size)
	if size > 0 {
		if _, err := io.ReadFull(c.rw, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
	}
	return &Frame{Op: op, Payload: payload}, nil
}
