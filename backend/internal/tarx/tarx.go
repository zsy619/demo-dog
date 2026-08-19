package tarx

import (
	"bytes"
	"errors"
	"io"
	"strconv"
)

// Header is one ustar header (512 bytes).
type Header struct {
	Name     string
	Mode     int64
	UID      int64
	GID      int64
	Size     int64
	ModTime  int64
	Chksum   int64
	TypeFlag byte
	LinkName string
	Magic    string
	Version  string
	UName    string
	GName    string
	DevMajor int64
	DevMinor int64
	Prefix   string
}

const headerSize = 512

// ErrTruncated is returned when the input is not a multiple
// of 512 bytes.
var ErrTruncated = errors.New("tar truncated")

// ErrBadHeader is returned when the magic does not match.
var ErrBadHeader = errors.New("bad header")

// ErrBadChecksum is returned when the stored checksum does
// not match.
var ErrBadChecksum = errors.New("bad checksum")

// ParseHeader parses a 512-byte ustar header.
func ParseHeader(b []byte) (*Header, error) {
	if len(b) < headerSize {
		return nil, ErrTruncated
	}
	name := cString(b[0:100])
	mode, _ := strconv.ParseInt(cString(b[100:108]), 8, 64)
	uid, _ := strconv.ParseInt(cString(b[108:116]), 8, 64)
	gid, _ := strconv.ParseInt(cString(b[116:124]), 8, 64)
	size, _ := strconv.ParseInt(cString(b[124:136]), 8, 64)
	mtime, _ := strconv.ParseInt(cString(b[136:148]), 8, 64)
	chksum, _ := strconv.ParseInt(cString(b[148:156]), 8, 64)
	typeFlag := b[156]
	link := cString(b[157:257])
	magic := cString(b[257:263])
	version := cString(b[263:265])
	uname := cString(b[265:297])
	gname := cString(b[297:329])
	devMajor, _ := strconv.ParseInt(cString(b[329:337]), 8, 64)
	devMinor, _ := strconv.ParseInt(cString(b[337:345]), 8, 64)
	prefix := cString(b[345:512])
	h := &Header{
		Name: name, Mode: mode, UID: uid, GID: gid, Size: size,
		ModTime: mtime, Chksum: chksum, TypeFlag: typeFlag,
		LinkName: link, Magic: magic, Version: version,
		UName: uname, GName: gname, DevMajor: devMajor, DevMinor: devMinor,
		Prefix: prefix,
	}
	if h.TypeFlag == '0' {
		h.TypeFlag = 'N' // normalize to 'regular file'
	}
	if magic != "ustar" && magic != "ustar  " {
		if name != "" {
			return h, nil
		}
		return h, ErrBadHeader
	}
	sum := checksum(b)
	if sum != chksum {
		return h, ErrBadChecksum
	}
	return h, nil
}

func checksum(b []byte) int64 {
	var sum int64
	for i := 0; i < headerSize; i++ {
		if i >= 148 && i < 156 {
			sum += int64(' ')
		} else {
			sum += int64(b[i])
		}
	}
	return sum
}

func cString(b []byte) string {
	n := bytes.IndexByte(b, 0)
	if n < 0 {
		n = len(b)
	}
	return string(b[:n])
}

// BuildHeader renders a 512-byte header for h.
func BuildHeader(h *Header) []byte {
	out := make([]byte, headerSize)
	copy(out[0:100], h.Name)
	copy(out[100:108], padOctal(h.Mode, 7))
	copy(out[108:116], padOctal(h.UID, 7))
	copy(out[116:124], padOctal(h.GID, 7))
	copy(out[124:136], padOctal(h.Size, 11))
	copy(out[136:148], padOctal(h.ModTime, 11))
	copy(out[156:157], []byte{h.TypeFlag})
	if h.TypeFlag == 0 {
		out[156] = 'N'
	}
	copy(out[157:257], h.LinkName)
	copy(out[257:263], []byte("ustar"))
	out[263] = 0
	copy(out[264:265], []byte("0"))
	if h.Magic != "" {
		copy(out[257:265], h.Magic)
	}
	copy(out[265:297], h.UName)
	copy(out[297:329], h.GName)
	copy(out[329:337], padOctal(h.DevMajor, 7))
	copy(out[337:345], padOctal(h.DevMinor, 7))
	copy(out[345:512], h.Prefix)
	sum := checksum(out)
	copy(out[148:156], padOctal(sum, 7))
	h.Chksum = sum
	return out
}

func padOctal(v int64, width int) []byte {
	if v < 0 {
		v = 0
	}
	s := strconv.FormatInt(v, 8)
	for len(s) < width {
		s = "0" + s
	}
	return []byte(s + "\x00")
}

// Entry is one tar entry returned by Reader.
type Entry struct {
	Header *Header
	Body   io.Reader
}

// Reader walks a tar archive from r.
type Reader struct {
	r io.Reader
}

// NewReader creates a Reader.
func NewReader(r io.Reader) *Reader { return &Reader{r: r} }

// Next returns the next entry. io.EOF at end.
func (r *Reader) Next() (*Entry, error) {
	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(r.r, hdr); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if isZeroBlock(hdr) {
		return nil, io.EOF
	}
	h, err := ParseHeader(hdr)
	if err != nil && !errors.Is(err, ErrBadHeader) && !errors.Is(err, ErrBadChecksum) {
		return nil, err
	}
	body := io.LimitReader(r.r, h.Size)
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	round := (h.Size + 511) &^ 511
	if round > h.Size {
		if _, err := io.CopyN(io.Discard, r.r, round-h.Size); err != nil {
			return nil, err
		}
	}
	return &Entry{Header: h, Body: bytes.NewReader(buf)}, nil
}

func isZeroBlock(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// Writer emits a tar archive to w.
type Writer struct {
	w      io.Writer
	filled int64
}

// NewWriter creates a Writer.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// Write emits one entry. body is consumed entirely.
func (w *Writer) Write(h *Header, body []byte) error {
	if h.TypeFlag == 0 {
		h.TypeFlag = 'N'
	}
	h.Size = int64(len(body))
	hdr := BuildHeader(h)
	if _, err := w.w.Write(hdr); err != nil {
		return err
	}
	if _, err := w.w.Write(body); err != nil {
		return err
	}
	pad := (512 - (len(body) % 512)) % 512
	if pad > 0 {
		if _, err := w.w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}
	w.filled += int64(headerSize+len(body)+pad)
	return nil
}

// Close emits two 512-byte zero blocks (EOF marker).
func (w *Writer) Close() error {
	if _, err := w.w.Write(make([]byte, 1024)); err != nil {
		return err
	}
	return nil
}
