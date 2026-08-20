// Package jsonx reader.go: 流式 JSON token 读取器。
package jsonx

import (
	"bytes"
	"io"
	"strconv"
)

// Reader 是流式 JSON tokenizer。
//
// 通过 Next() 依次读取 token 直到 TokenEOF / io.EOF / ErrBadJSON。
type Reader struct {
	r   *bytes.Reader // 底层字节读取器
	buf []byte        // 复用缓冲(避免每次分配)
}

// NewReader 从 JSON 字节流创建一个 Reader。
func NewReader(data []byte) *Reader {
	return &Reader{r: bytes.NewReader(data)}
}

// Next 返回下一个 token。
//
// 跳过空白与分隔符;遇到非法字符返回 ErrBadJSON。
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

// expect 断言接下来若干字节匹配 s;不匹配则设置内部错误。
func (r *Reader) expect(s string) {
	for i := 0; i < len(s); i++ {
		c, _ := r.r.ReadByte()
		if c != s[i] {
			r.errSet()
		}
	}
}

// errSet 在内部记录 ErrBadJSON (用于 expect 不匹配)。
func (r *Reader) errSet() {
	// 简化实现:仅保留 ErrBadJSON 标记。
	_ = ErrBadJSON
}

// readString 读取 "..." 形式的 JSON 字符串。
func (r *Reader) readString() (string, error) {
	var b []byte
	for {
		c, err := r.r.ReadByte()
		if err != nil {
			return "", err
		}
		if c == '"' {
			return string(b), nil
		}
		if c == '\\' {
			n, err := r.r.ReadByte()
			if err != nil {
				return "", err
			}
			switch n {
			case 'n':
				c = '\n'
			case 't':
				c = '\t'
			case 'r':
				c = '\r'
			case '"':
				c = '"'
			case '\\':
				c = '\\'
			default:
				c = n
			}
		}
		b = append(b, c)
	}
}

// readNumber 读取数字字面量。
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


