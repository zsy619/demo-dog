// Package jsonx JSON 扩展:增量编解码、字段合并、流式解析。
//
// 本包按类型拆分到多个文件:
//   - writer.go  Writer 类型与所有写方法
//   - reader.go  Reader 类型与所有读方法
//   - token.go   Token / TokenType / ErrBadJSON
package jsonx

import (
	"io"
	"strconv"
)

// Writer 以增量方式写出 JSON token。
//
// 典型用法: NewWriter(buf).Object().Key("a").Int(1).Key("b").String("x").EndObject()。
type Writer struct {
	w   io.Writer // 底层写入目标
	err error     // 首次遇到的错误(首次错误后写操作 no-op)
}

// NewWriter 用给定的 io.Writer 构造一个 Writer。
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Err 返回首次遇到的错误。
func (w *Writer) Err() error { return w.err }

// write 内部写入,遇到错误后所有调用静默失败。
func (w *Writer) write(p []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.Write(p)
}

// Object 输出 {。
func (w *Writer) Object() {
	w.write([]byte{'{'})
}

// EndObject 输出 }。
func (w *Writer) EndObject() {
	w.write([]byte{'}'})
}

// Array 输出 [。
func (w *Writer) Array() {
	w.write([]byte{'['})
}

// EndArray 输出 ]。
func (w *Writer) EndArray() {
	w.write([]byte{']'})
}

// Comma 输出 ,。
func (w *Writer) Comma() {
	w.write([]byte{','})
}

// Key 输出 "key": 形式(带引号和冒号)。
func (w *Writer) Key(s string) {
	w.write([]byte{'"'})
	w.writeString(s)
	w.write([]byte{'"', ':'})
}

// String 输出 "value" 形式的 JSON 字符串(含转义)。
func (w *Writer) String(s string) {
	w.write([]byte{'"'})
	w.writeString(s)
	w.write([]byte{'"'})
}

// writeString 写出单个字符串字面量,并对特殊字符进行转义。
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

// Int 输出整型字面量。
func (w *Writer) Int(v int64) {
	w.write([]byte(strconv.FormatInt(v, 10)))
}

// Uint 输出无符号整型字面量。
func (w *Writer) Uint(v uint64) {
	w.write([]byte(strconv.FormatUint(v, 10)))
}

// Float 输出浮点字面量(使用 -1 精度以便自动选择最短表示)。
func (w *Writer) Float(v float64) {
	w.write([]byte(strconv.FormatFloat(v, 'g', -1, 64)))
}

// Bool 输出 true 或 false。
func (w *Writer) Bool(v bool) {
	if v {
		w.write([]byte{'t', 'r', 'u', 'e'})
	} else {
		w.write([]byte{'f', 'a', 'l', 's', 'e'})
	}
}

// Null 输出 null。
func (w *Writer) Null() {
	w.write([]byte{'n', 'u', 'l', 'l'})
}

// Raw 直接写入一段预先生成的 JSON 字节(不会进行转义校验)。
func (w *Writer) Raw(p []byte) {
	w.write(p)
}
