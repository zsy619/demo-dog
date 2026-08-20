// Package jsonx JSON 扩展：增量编解码、字段合并、流式解析。
package jsonx

import (
	"bytes"
	"errors"
	"io"
	"strconv"
)

// Writer writes JSON tokens incrementally.
type Writer struct {
	w   io.Writer
	err error
}

// NewWriter wraps w with a JSON Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Err returns the first error encountered.
func (w *Writer) Err() error { return w.err }

func (w *Writer) write(p []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.Write(p)
}

// Object opens an object.
func (w *Writer) Object() {
	w.write([]byte{'{'})
}

// EndObject closes an object.
func (w *Writer) EndObject() {
	w.write([]byte{'}'})
}

// Array opens an array.
func (w *Writer) Array() {
	w.write([]byte{'['})
}

// EndArray closes an array.
func (w *Writer) EndArray() {
	w.write([]byte{']'})
}

// Comma writes a comma separator.
func (w *Writer) Comma() {
	w.write([]byte{','})
}

// Key writes a string key with surrounding quotes + colon.
func (w *Writer) Key(s string) {
	w.write([]byte{'"'})
	w.writeString(s)
	w.write([]byte{'"', ':'})
}

// String writes a JSON string value.
func (w *Writer) String(s string) {
	w.write([]byte{'"'})
	w.writeString(s)
	w.write([]byte{'"'})
}

func (w *Writer) writeString(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			w.write([]byte{'\\', c})
		case '\n':
			w.write([]byte{'\\', 'n'})
		case '\r':
			w.write([]byte{'\\', 'r'})
		case '\t':
			w.write([]byte{'\\', 't'})
		default:
			if c < 0x20 {
				w.write([]byte{'\\', 'u', '0', '0', '0', '0'})
				continue
			}
			w.write([]byte{c})
		}
	}
}

// Int writes an integer.
func (w *Writer) Int(v int64) {
	w.write([]byte(strconv.FormatInt(v, 10)))
}

// Uint writes an unsigned integer.
func (w *Writer) Uint(v uint64) {
	w.write([]byte(strconv.FormatUint(v, 10)))
}

// Float writes a float.
func (w *Writer) Float(v float64) {
	w.write([]byte(strconv.FormatFloat(v, 'g', -1, 64)))
}

// Bool writes a boolean.
func (w *Writer) Bool(v bool) {
	if v {
		w.write([]byte{'t', 'r', 'u', 'e'})
	} else {
		w.write([]byte{'f', 'a', 'l', 's', 'e'})
	}
}

// Null writes null.
func (w *Writer) Null() {
	w.write([]byte{'n', 'u', 'l', 'l'})
}

// Raw writes raw JSON bytes verbatim.
func (w *Writer) Raw(p []byte) {
	w.write(p)
}

// Reader is a streaming JSON tokenizer.
type Reader struct {
	r   *bytes.Reader
	buf []byte
}

// NewReader creates a streaming reader from JSON bytes.
func NewReader(data []byte) *Reader {
	return &Reader{r: bytes.NewReader(data)}
}

// TokenType is one of the JSON token kinds.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenObjectStart
	TokenObjectEnd
	TokenArrayStart
	TokenArrayEnd
	TokenString
	TokenNumber
	TokenBool
	TokenNull
)

// ErrBadJSON is returned on syntax errors.
var ErrBadJSON = errors.New("bad json")

// Token is one parsed value.
type Token struct {
	Type  TokenType
	Str   string
	Num   float64
	Bool  bool
}

// Next returns the next token.
func (r *Reader) Next() (Token, error) {
	for {
		c, err := r.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return Token{Type: TokenEOF}, io.EOF
			}
			return Token{}, err
		}
		switch c {
		case ' ', '\n', '\r', '\t', ':', ',':
			continue
		case '{':
			return Token{Type: TokenObjectStart}, nil
		case '}':
			return Token{Type: TokenObjectEnd}, nil
		case '[':
			return Token{Type: TokenArrayStart}, nil
		case ']':
			return Token{Type: TokenArrayEnd}, nil
		case '"':
			str, err := r.readString()
			if err != nil {
				return Token{}, err
			}
			return Token{Type: TokenString, Str: str}, nil
		case 't', 'f':
			if c == 't' {
				r.expect("rue")
				return Token{Type: TokenBool, Bool: true}, nil
			}
			r.expect("alse")
			return Token{Type: TokenBool, Bool: false}, nil
		case 'n':
			r.expect("ull")
			return Token{Type: TokenNull}, nil
		default:
			if c == '-' || (c >= '0' && c <= '9') {
				num, err := r.readNumber(c)
				if err != nil {
					return Token{}, err
				}
				return Token{Type: TokenNumber, Num: num}, nil
			}
			return Token{}, ErrBadJSON
		}
	}
}

func (r *Reader) expect(s string) {
	for i := 0; i < len(s); i++ {
		c, _ := r.r.ReadByte()
		if c != s[i] {
			r.errSet()
		}
	}
}

func (r *Reader) errSet() {
	r.r.Seek(0, 2) // force EOF on next read
}

func (r *Reader) readString() (string, error) {
	var b []byte
	for {
		c, err := r.r.ReadByte()
		if err != nil {
			return "", ErrBadJSON
		}
		if c == '"' {
			return string(b), nil
		}
		if c == '\\' {
			c, err = r.r.ReadByte()
			if err != nil {
				return "", ErrBadJSON
			}
			switch c {
			case 'n':
				b = append(b, '\n')
			case 'r':
				b = append(b, '\r')
			case 't':
				b = append(b, '\t')
			default:
				b = append(b, c)
			}
			continue
		}
		b = append(b, c)
	}
}

func (r *Reader) readNumber(first byte) (float64, error) {
	var b []byte
	b = append(b, first)
	for {
		c, err := r.r.ReadByte()
		if err != nil {
			break
		}
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			b = append(b, c)
			continue
		}
		r.r.UnreadByte()
		break
	}
	return strconv.ParseFloat(string(b), 64)
}
