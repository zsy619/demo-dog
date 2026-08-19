// Package hexdump 提供类似 xxd/hexdump 的格式化输出。
package hexdump

import (
	"fmt"
	"io"
)

// Dump 把数据以十六进制 + ASCII 形式写入 w。
func Dump(w io.Writer, data []byte) {
	const line = 16
	for i := 0; i < len(data); i += line {
		end := i + line
		if end > len(data) {
			end = len(data)
		}
		fmt.Fprintf(w, "%08x  ", i)
		for j := 0; j < line; j++ {
			if j+i < end {
				fmt.Fprintf(w, "%02x ", data[i+j])
			} else {
				io.WriteString(w, "   ")
			}
			if j == 7 {
				io.WriteString(w, " ")
			}
		}
		io.WriteString(w, " ")
		for j := 0; j < line && j+i < end; j++ {
			c := data[i+j]
			if c >= 32 && c < 127 {
				fmt.Fprintf(w, "%c", c)
			} else {
				io.WriteString(w, ".")
			}
		}
		io.WriteString(w, "\n")
	}
}

// ToString 返回 Dump 的字符串形式。
func ToString(data []byte) string {
	var sb []byte
	buf := writerBuf{p: &sb}
	Dump(&buf, data)
	return string(sb)
}

type writerBuf struct {
	p *[]byte
}

func (b writerBuf) Write(p []byte) (int, error) {
	*b.p = append(*b.p, p...)
	return len(p), nil
}
