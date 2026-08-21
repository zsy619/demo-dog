// WebSocket 升级 + 单帧写入辅助。
//
// Hand-rolled on top of crypto/sha1 + encoding/base64 (RFC 6455) so the demo
// has zero external dependencies.
package stream

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
)

var acceptKeyGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Upgrade 在被劫持的 HTTP 连接上执行 WebSocket 握手。
// It 返回 a connection that yields text frames via Read and 写入 them
// via WriteText.
//
// allowedOrigins controls the cross-site check. 空的 list accepts any
// origin (dev mode); passing a non-empty list rejects requests whose
// Origin header does not match one of the entries. Same-origin requests
// (no Origin header, or Origin == Host) are always accepted.
func Upgrade(w http.ResponseWriter, r *http.Request, allowedOrigins []string) (net.Conn, *Conn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, nil, errors.New("not a websocket upgrade")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, nil, errors.New("missing Sec-WebSocket-Key")
	}
	if origin := r.Header.Get("Origin"); origin != "" && !originAllowed(origin, allowedOrigins) {
		return nil, nil, errors.New("origin not allowed: " + origin)
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacker not supported")
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}
	resp := []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ")
	sum := sha1.Sum([]byte(key + acceptKeyGUID))
	resp = append(resp, base64.StdEncoding.EncodeToString(sum[:])...)
	resp = append(resp, []byte("\r\n\r\n")...)
	if _, err := bufrw.Write(resp); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, &Conn{rwc: conn, bufrw: bufrw}, nil
}

// originAllowed 当 `origin` 是 allowedOrigins 之一时返回 true，
// or if allowedOrigins is empty (dev 默认值). A wildcard entry "*"
// also matches any origin.
func originAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return true
	}
	for _, a := range allowedOrigins {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}

// Conn 包装被劫持的连接并提供 WebSocket 帧处理。
type Conn struct {
	rwc   net.Conn
	bufrw interface{ Flush() error }

	closed bool
}

// ReadFrame 返回下一个文本或二进制帧的负载。
// Close frames return io.EOF.
func (c *Conn) ReadFrame() ([]byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(c.rwc, hdr[:]); err != nil {
		return nil, err
	}
	fin := hdr[0]&0x80 != 0
	opcode := hdr[0] & 0x0F
	masked := hdr[1]&0x80 != 0
	length := int64(hdr[1] & 0x7F)

	switch {
	case length == 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.rwc, ext[:]); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext[:]))
	case length == 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.rwc, ext[:]); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint64(ext[:]))
	}
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.rwc, maskKey[:]); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.rwc, buf); err != nil {
		return nil, err
	}
	if masked {
		for i := range buf {
			buf[i] ^= maskKey[i%4]
		}
	}
	if opcode == 0x8 {
		// close 帧
		return nil, io.EOF
	}
	_ = fin
	return buf, nil
}

// WriteText 写入单个文本帧。
func (c *Conn) WriteText(payload []byte) error {
	if c.closed {
		return errors.New("connection closed")
	}
	var hdr [2]byte
	hdr[0] = 0x81 // FIN + text
	n := len(payload)
	switch {
	case n < 126:
		hdr[1] = byte(n)
		if _, err := c.rwc.Write(hdr[:]); err != nil {
			return err
		}
	case n < 65536:
		hdr[1] = 126
		if _, err := c.rwc.Write(hdr[:]); err != nil {
			return err
		}
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(n))
		if _, err := c.rwc.Write(ext[:]); err != nil {
			return err
		}
	default:
		hdr[1] = 127
		if _, err := c.rwc.Write(hdr[:]); err != nil {
			return err
		}
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		if _, err := c.rwc.Write(ext[:]); err != nil {
			return err
		}
	}
	if _, err := c.rwc.Write(payload); err != nil {
		return err
	}
	if f, ok := c.bufrw.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// Close 发送 close 帧并关闭底层连接。
func (c *Conn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	// close 帧：0x88 0x00
	_, _ = c.rwc.Write([]byte{0x88, 0x00})
	if f, ok := c.bufrw.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	return c.rwc.Close()
}
