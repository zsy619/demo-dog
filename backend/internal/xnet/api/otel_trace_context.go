package api

import (
	"net/http"
	"regexp"
	"strings"
)

// W3C Trace Context（https://www.w3.org/TR/trace-context/）解析器。
//
// `traceparent` 头部格式如下：
//
//   version-traceid-spanid-flags
//
// 其中 version 是两个十六进制字符，traceid 是 32 个十六进制字符，
// spanid 是 16 个十六进制字符，flags 是两个十六进制字符（位 0 = 已采样）。
//
// 示例：
//
//   traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
//
// `tracestate` 是一个以逗号分隔的厂商特定
// key=value 对列表，用于补充 trace。我们按原样接受
// 并重新发出。

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

// ParseTraceContext 从请求头中提取 W3C trace context。
// 如果缺失或无效则返回 nil，调用方可以
// 回退到生成一个新的 context。
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
		// flags 字节编码为两个十六进制字符：flags[0] 是高位半字节，
		// flags[1] 是低位半字节。已采样标志是字节的位 0。
		// 
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

// GenerateTraceContext 返回一对新的 trace + span id。
// 用于请求未携带传入 context、但仍希望
// 将其附加到响应上的情况。
func GenerateTraceContext() *TraceContext {
	return &TraceContext{
		Version: "00",
		TraceID: randomHex(32),
		SpanID:  randomHex(16),
		Flags:   "01",
		Sampled: true,
	}
}

// childSpanID 生成一个新的 16 字符十六进制的 span id；
// 抽出此辅助函数以便 trace 传播代码可自顶向下阅读。
func childSpanID() string { return randomHex(16) }

// randomHex 通过 stdlib 的 crypto/rand 生成 n 个十六进制字符。
// 这里 import 是为了使 trace context 生成代码集中在此文件。
func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	// 通过 stdlib 使用 crypto/rand。内联导入避免
	// 与 api 包产生导入循环。
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
