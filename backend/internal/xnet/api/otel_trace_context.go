package api

import (
	"net/http"
	"regexp"
	"strings"
)

// W3C Trace Context (https://www.w3.org/TR/trace-context/) 解析器。
// 
// `traceparent` 头部的格式为：
// 
// version-traceid-spanid-flags
// 
// 其中 version 是两个十六进制字符，traceid 是 32 个十六进制字符，
// spanid 是 16 个十六进制字符，flags 是两个十六进制字符
// (bit 0 = sampled)。
// 
// 示例：
// 
// traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
// 
// `tracestate` 是逗号分隔的厂商特定
// key=value 对列表，用于扩充 trace。我们原样接受并重新输出。

type TraceContext struct {
	Version    string
	TraceID    string
	SpanID     string
	Flags      string
	Tracestate string
	Sampled    bool
}

var (
	tpidRE      = regexp.MustCompile(`^([0-9a-fA-F]{2})-([0-9a-fA-F]{32})-([0-9a-fA-F]{16})-([0-9a-fA-F]{2})$`)
	invalidHex  = regexp.MustCompile(`^0+$`)
)

// ParseTraceContext 从请求头部提取 W3C trace context。
// 当缺失或无效时返回 nil；调用方可以回退到
// 生成一个新的 context。
func ParseTraceContext(r *http.Request) *TraceContext {
	tp := strings.TrimSpace(r.Header.Get("traceparent"))
	if tp == "" {
		return nil
	}
	m := tpidRE.FindStringSubmatch(tp)
	if m == nil {
		return nil
	}
	traceID := strings.ToLower(m[2])
	spanID := strings.ToLower(m[3])
	// 全零的 trace_id 或 span_id 依规范无效。
	if invalidHex.MatchString(traceID) || invalidHex.MatchString(spanID) {
		return nil
	}
	flags := m[4]
	sampled := false
	// 第一个半字节的位 0 是已采样标志。
	if len(flags) >= 1 {
		// flags 字节以两个十六进制字符编码：flags[0] 是
		// 高位 nibble，flags[1] 是低位 nibble。
		// sampled 标志是该字节的 bit 0。
		hi := hexNibble(flags[0])
		lo := hexNibble(flags[1])
		if hi >= 0 && lo >= 0 && ((hi<<4 | lo) & 1) == 1 {
			sampled = true
		}
	}
	return &TraceContext{
		Version:    m[1],
		TraceID:    traceID,
		SpanID:     spanID,
		Flags:      flags,
		Tracestate: r.Header.Get("tracestate"),
		Sampled:    sampled,
	}
}

// InjectTraceContext 将 W3C trace context 写入响应头，
// 以便下游调用方将其 span 拼接到同一 trace。
// 
func InjectTraceContext(rw http.ResponseWriter, tc *TraceContext) {
	if tc == nil {
		return
	}
	traceparent := tc.Version + "-" + tc.TraceID + "-" + tc.SpanID + "-" + tc.Flags
	rw.Header().Set("traceparent", traceparent)
	if tc.Tracestate != "" {
		rw.Header().Set("tracestate", tc.Tracestate)
	}
}

// GenerateTraceContext 返回一对全新的 trace + span id。用于
// 请求没有传入 context，但仍希望
// 给响应附加一个的场景。
func GenerateTraceContext() *TraceContext {
	return &TraceContext{
		Version: "00",
		TraceID: randomHex(32),
		SpanID:  randomHex(16),
		Flags:   "01",
		Sampled: true,
	}
}

// childSpanID 生成一个全新的 16 位十六进制 span id；提供该辅助函数是为了让
// trace 传播相关代码可以自顶向下阅读。
func childSpanID() string { return randomHex(16) }

// randomHex 通过 stdlib 从 crypto/rand 生成 n 个十六进制字符。
// 我们在此处导入，以便 trace context 的生成逻辑都集中在本文件中。
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	// 通过 stdlib 使用 crypto/rand。内联导入可以避免与 api 包之间
	// 产生循环导入。
	r := cryptoRandBytes((n + 1) / 2)
	for i := range b {
		if i%2 == 0 {
			b[i] = hex[r[i/2]>>4]
		} else {
			b[i] = hex[r[i/2]&0x0f]
		}
	}
	return string(b)
}

// hexNibble 返回十六进制字符对应的 0..15，若不是十六进制字符则返回 -1。
func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
