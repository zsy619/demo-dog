// Package diffx 提供简单的行级 diff 工具：
// 基于 LCS 的最长公共子序列，返回差异段。
package diffx

import "strings"

// Op 是差异操作类型。
type Op int

const (
	OpEq Op = iota
	OpDel
	OpAdd
)

// Chunk 是一次差异结果。
type Chunk struct {
	Op  Op
	Old string
	New string
}

// DiffLine 计算 a, b 两个字符串的逐行差异。
func DiffLine(a, b string) []Chunk {
	aLines := splitLines(a)
	bLines := splitLines(b)
	lcs := lcsTable(aLines, bLines)
	return buildChunks(aLines, bLines, lcs)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func lcsTable(a, b []string) [][]int {
	n, m := len(a), len(b)
	t := make([][]int, n+1)
	for i := range t {
		t[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				t[i][j] = t[i-1][j-1] + 1
			} else if t[i-1][j] >= t[i][j-1] {
				t[i][j] = t[i-1][j]
			} else {
				t[i][j] = t[i][j-1]
			}
		}
	}
	return t
}

func buildChunks(a, b []string, t [][]int) []Chunk {
	var out []Chunk
	i, j := len(a), len(b)
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			out = append(out, Chunk{Op: OpEq, Old: a[i-1], New: b[j-1]})
			i--
			j--
		case j > 0 && (i == 0 || t[i][j-1] >= t[i-1][j]):
			out = append(out, Chunk{Op: OpAdd, New: b[j-1]})
			j--
		default:
			out = append(out, Chunk{Op: OpDel, Old: a[i-1]})
			i--
		}
	}
	// 反转
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}

// Unified 渲染一个统一的 diff 文本。
func Unified(chunks []Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		switch c.Op {
		case OpEq:
			b.WriteString("  ")
			b.WriteString(c.Old)
			b.WriteString("\n")
		case OpDel:
			b.WriteString("- ")
			b.WriteString(c.Old)
			b.WriteString("\n")
		case OpAdd:
			b.WriteString("+ ")
			b.WriteString(c.New)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Stat 返回 +/- 行数。
func Stat(chunks []Chunk) (add, del int) {
	for _, c := range chunks {
		switch c.Op {
		case OpAdd:
			add++
		case OpDel:
			del++
		}
	}
	return
}
