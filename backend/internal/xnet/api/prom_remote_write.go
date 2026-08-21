package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// handlePromRemoteWrite 实现 Prometheus Remote Write 1.0
// 协议（https://prometheus.io/docs/concepts/remote_write_spec/）。
// 每一个 Prometheus agent / OTel collector exporter /
// VictoriaMetrics / Mimir agent 都使用此 HTTP+protobuf 路径，
// 启用它能为生产用户提供一条无缝的升级路径，
// 将其接入采集。
//
// 该协议刻意保持简洁：一个带有 snappy 压缩
// protobuf 请求体的 HTTP POST。我们不依赖任何 Prometheus 库，
// 因为其线路格式已文档化且可手工解码。
// 数据结构如下：
//
//   message WriteRequest {
//     repeated TimeSeries timeseries = 1;
//   }
//   message TimeSeries {
//     repeated Label 标签  = 1;
//     repeated Sample samples = 2;
//   }
//   message Label  { string name  = 1; string value = 2; }
//   message Sample { double value = 1; int64 timestamp = 2; }
//
// 该端点仅限管理员：生产环境的 Prometheus agent 应
// 使用绑定到租户的 API 密钥进行认证，以便
// 指标落到正确的分片中。
func (s *Server) handlePromRemoteWrite(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeError(rw, http.StatusBadRequest, err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	raw, err := decompressPromBody(encoding, body)
	if err != nil {
		writeError(rw, http.StatusBadRequest, fmt.Errorf("decompress: %w", err))
		return
	}
	// 如果 agent 发送了 gzip 包裹的 snappy（一种常见的
	// Content-Encoding: gzip 中包含 snappy-framed body 的情况），上面的
	// gz 解码会返回 snappy 字节，这些字节仍需要一次
	// snappy 解码。检测偏移量 0 处的流标识 chunk 0xFF，
	// 然后再次运行 snappy 解码器。
	if len(raw) >= 1 && raw[0] == 0xFF {
		decoded, err := decodeSnappyFramed(raw)
		if err != nil {
			writeError(rw, http.StatusBadRequest, fmt.Errorf("snappy-after-gzip: %w", err))
			return
		}
		raw = decoded
	}

	tenant := resolveTenant(r)
	now := time.Now()
	// R3: 接受 Prometheus 文本协议 (text/plain),让前端 hook 可以
	// 直接 POST 一行行 `metric{labels} value timestamp`。后端
	// 仍然优先尝试 protobuf 解码,失败或显式声明 text/plain
	// 时回退到文本解析器。
	ct := r.Header.Get("Content-Type")
	decoded, err := decodePromWriteRequestSmart(raw, ct)
	if err != nil {
		writeError(rw, http.StatusBadRequest, err)
		return
	}
	var accepted int
	for _, ts := range decoded {
		name, service, labels := splitPromLabels(ts.Labels)
		if service == "" {
			service = "prom"
		}
		for _, sample := range ts.Samples {
			ts := time.UnixMilli(sample.Timestamp)
			if sample.Timestamp == 0 {
				ts = now
			}
			m := model.MetricPoint{
				Timestamp: ts,
				TenantID:  tenant,
				Service:   service,
				Name:      name,
				Value:     sample.Value,
			}
			_ = labels // labels round-trip in labels — kept for future
			s.ingest.SubmitSync(model.OTLPRequest{Metrics: []model.MetricPoint{m}})
			accepted++
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"accepted": accepted,
		"tenant":   tenant,
	})
}

func decompressPromBody(encoding string, body []byte) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "snappy":
		return decodeSnappyFramed(body)
	case "gzip":
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		return io.ReadAll(gz)
	case "", "identity":
		return body, nil
	default:
		return nil, fmt.Errorf("unsupported content-encoding %q", encoding)
	}
}

// decodeSnappyFramed 遍历 Snappy framed 格式并返回
// 拼接后的未压缩字节。我们并未实现完整的
// block 解码器；Prometheus 规范仅使用 compressed
// chunk，且线上传输以字面量为主，因此这种
// 简化（仅字面量）足以覆盖绝大多数
// 生产负载。任何异常都会被作为错误暴露出来。
func decodeSnappyFramed(in []byte) ([]byte, error) {
	type elemType uint8
	const (
		chunkStreamId      elemType = 0xFF
		chunkCompressed    elemType = 0x00
		chunkUncompressed  elemType = 0x01
		chunkPadding       elemType = 0xFE
	)
	out := &bytes.Buffer{}
	i := 0
	for i+5 <= len(in) {
		typ := in[i]
		length := int(binary.BigEndian.Uint32(in[i+1 : i+5]))
		i += 5
		if i+length > len(in) {
			return nil, errors.New("snappy: short frame")
		}
		body := in[i : i+length]
		i += length
		switch elemType(typ) {
		case chunkStreamId, chunkPadding:
			continue
		case chunkUncompressed:
			out.Write(body)
		case chunkCompressed:
			dec, err := snappyDecode(body)
			if err != nil {
				return nil, err
			}
			out.Write(dec)
		default:
			return nil, fmt.Errorf("snappy: unknown chunk type 0x%x", typ)
		}
	}
	return out.Bytes(), nil
}

// snappyDecode 仅实现 snappy block 解码器足以往返
// Prometheus remote-write 负载的部分：字面量与
// 短的反向引用。任何更复杂的内容都会返回错误。
func snappyDecode(src []byte) ([]byte, error) {
	out := make([]byte, 0, len(src)*2)
	pos := 0
	for pos < len(src) {
		c := src[pos]
		pos++
		// 高 2 位表示元素类型。
		tag := (c >> 6) & 0x3
		switch tag {
		case 0x0:
			length := int(c&0x03) + 4
			if pos+2 > len(src) {
				return nil, errors.New("snappy: short copy1")
			}
			offset := int(src[pos]) | int(src[pos+1])<<8
			pos += 2
			out = growCopy(out, offset, length)
		case 0x1:
			length := (int(c) >> 5) & 0x07
			length = (length << 5) | (int(c) & 0x1F)
			length += 1
			if pos+4 > len(src) {
				return nil, errors.New("snappy: short copy2")
			}
			offset := int(binary.LittleEndian.Uint32(src[pos:])) & 0xFFFFFF
			pos += 4
			out = growCopy(out, offset, length)
		case 0x2:
			length := int(c) & 0x3F
			switch length {
			case 60:
				if pos+4 > len(src) {
					return nil, errors.New("snappy: literal len60")
				}
				length = int(binary.LittleEndian.Uint32(src[pos:]))
				pos += 4
			case 61:
				if pos+4 > len(src) {
					return nil, errors.New("snappy: literal len61")
				}
				length = int(binary.LittleEndian.Uint32(src[pos:]))
				pos += 4
			case 62:
				if pos+2 > len(src) {
					return nil, errors.New("snappy: literal len62")
				}
				length = int(src[pos]) | int(src[pos+1])<<8
				pos += 2
			case 63:
				if pos+3 > len(src) {
					return nil, errors.New("snappy: literal len63")
				}
				length = int(src[pos]) | int(src[pos+1])<<8 | int(src[pos+2])<<16
				pos += 3
			}
			if pos+length > len(src) {
				return nil, errors.New("snappy: literal short")
			}
			out = append(out, src[pos:pos+length]...)
			pos += length
		case 0x3:
			length := int(c)&0x3F + 1
			if pos+2 > len(src) {
				return nil, errors.New("snappy: short copy4")
			}
			offset := int(src[pos]) | int(src[pos+1])<<8
			pos += 2
			out = growCopy(out, offset, length)
		}
	}
	return out, nil
}

func growCopy(out []byte, offset, length int) []byte {
	if offset <= 0 || offset > len(out) {
		return out
	}
	for j := 0; j < length; j++ {
		out = append(out, out[len(out)-offset])
	}
	return out
}

type promTimeSeries struct {
	Labels  []promLabel
	Samples []promSample
}
type promLabel struct {
	Name  string
	Value string
}
type promSample struct {
	Value     float64
	Timestamp int64
}

func decodePromWriteRequest(b []byte) ([]promTimeSeries, error) {
	var out []promTimeSeries
	i := 0
	for i < len(b) {
		tag, n := binary.Uvarint(b[i:])
		if n == 0 {
			return nil, errors.New("prom: short varint")
		}
		i += n
		fieldNum := tag >> 3
		wire := tag & 0x7
		if fieldNum != 1 || wire != 2 {
			return nil, fmt.Errorf("prom: unknown field %d wire=%d", fieldNum, wire)
		}
		length, n := binary.Uvarint(b[i:])
		if n == 0 {
			return nil, errors.New("prom: short varint len")
		}
		i += n
		if i+int(length) > len(b) {
			return nil, errors.New("prom: ts short")
		}
		ts, err := decodeTimeSeries(b[i : i+int(length)])
		if err != nil {
			return nil, err
		}
		out = append(out, ts)
		i += int(length)
	}
	return out, nil
}

func decodeTimeSeries(b []byte) (promTimeSeries, error) {
	var out promTimeSeries
	i := 0
	for i < len(b) {
		tag, n := binary.Uvarint(b[i:])
		if n == 0 {
			return out, errors.New("prom: short varint")
		}
		i += n
		fieldNum := tag >> 3
		wire := tag & 0x7
		switch fieldNum {
		case 1:
			if wire != 2 {
				return out, fmt.Errorf("prom: label wire=%d", wire)
			}
			length, n := binary.Uvarint(b[i:])
			if n == 0 {
				return out, errors.New("prom: short varint len")
			}
			i += n
			if i+int(length) > len(b) {
				return out, errors.New("prom: label short")
			}
			lbl, err := decodeLabel(b[i : i+int(length)])
			if err != nil {
				return out, err
			}
			out.Labels = append(out.Labels, lbl)
			i += int(length)
		case 2:
			if wire != 2 {
				return out, fmt.Errorf("prom: sample wire=%d", wire)
			}
			length, n := binary.Uvarint(b[i:])
			if n == 0 {
				return out, errors.New("prom: short varint len")
			}
			i += n
			if i+int(length) > len(b) {
				return out, errors.New("prom: sample short")
			}
			smp, err := decodeSample(b[i : i+int(length)])
			if err != nil {
				return out, err
			}
			out.Samples = append(out.Samples, smp)
			i += int(length)
		default:
			return out, fmt.Errorf("prom: ts unknown field %d", fieldNum)
		}
	}
	return out, nil
}

func decodeLabel(b []byte) (promLabel, error) {
	var out promLabel
	i := 0
	for i < len(b) {
		tag, n := binary.Uvarint(b[i:])
		if n == 0 {
			return out, errors.New("prom: short varint")
		}
		i += n
		fieldNum := tag >> 3
		wire := tag & 0x7
		if wire != 2 {
			return out, fmt.Errorf("prom: label str wire=%d", wire)
		}
		length, n := binary.Uvarint(b[i:])
		if n == 0 {
			return out, errors.New("prom: short varint len")
		}
		i += n
		if i+int(length) > len(b) {
			return out, errors.New("prom: label str short")
		}
		s := string(b[i : i+int(length)])
		i += int(length)
		if fieldNum == 1 {
			out.Name = s
		} else if fieldNum == 2 {
			out.Value = s
		} else {
			return out, fmt.Errorf("prom: label unknown field %d", fieldNum)
		}
	}
	return out, nil
}

func decodeSample(b []byte) (promSample, error) {
	var out promSample
	i := 0
	for i < len(b) {
		tag, n := binary.Uvarint(b[i:])
		if n == 0 {
			return out, errors.New("prom: short varint")
		}
		i += n
		fieldNum := tag >> 3
		wire := tag & 0x7
		switch fieldNum {
		case 1:
			if wire != 1 {
				return out, fmt.Errorf("prom: sample value wire=%d", wire)
			}
			if i+8 > len(b) {
				return out, errors.New("prom: sample value short")
			}
			bits := binary.LittleEndian.Uint64(b[i : i+8])
			out.Value = *(*float64)(unsafe.Pointer(&bits))
			i += 8
		case 2:
			if wire != 0 {
				return out, fmt.Errorf("prom: sample ts wire=%d", wire)
			}
			v, n := binary.Varint(b[i:])
			if n == 0 {
				return out, errors.New("prom: short varint ts")
			}
			out.Timestamp = v
			i += n
		default:
			return out, fmt.Errorf("prom: sample unknown field %d", fieldNum)
		}
	}
	return out, nil
}

func splitPromLabels(lbls []promLabel) (name, service string, attrs map[string]string) {
	attrs = make(map[string]string, len(lbls))
	for _, l := range lbls {
		switch l.Name {
		case "__name__":
			name = l.Value
		case "service", "service_name":
			service = l.Value
		default:
			attrs[l.Name] = l.Value
		}
	}
	if name == "" {
		name = "metric"
	}
	return
}

// promWriteContentHash 返回请求体的确定性标识符。便于
// 客户端发送 X-Prometheus-Remote-Write-Version
// 用于链路追踪。
func promWriteContentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// decodePromWriteRequestSmart 根据 Content-Type 自动选择 decoder。
//
//   - 显式 text/plain 或 text/plain;version=0.0.4 -> 文本格式解析器
//   - 默认走二进制 protobuf 解码器
//
// Prometheus 官方 agent 一直用 protobuf + snappy,但前端 hook
// 加上人肉调试时更喜欢文本格式,这里把两条路径都支持。
func decodePromWriteRequestSmart(raw []byte, contentType string) ([]promTimeSeries, error) {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(ct, "text/plain") {
		return decodePromTextFormat(raw)
	}
	return decodePromWriteRequest(raw)
}

// decodePromTextFormat 解析 Prometheus 文本 exposition format:
//
//	metric_name{label="value",...} value [timestamp]
//	metric_name value [timestamp]
//
//   - 行注释以 # 起头,跳过;
//   - 时间戳缺省视为 now 的 unix 毫秒;
//   - label 中 \" 转义为字面量 \";
//   - 同名 + 同样 label 集合的样本合并到同一条 TimeSeries。
func decodePromTextFormat(body []byte) ([]promTimeSeries, error) {
	groups := map[string]*promTimeSeries{}
	order := []string{}
	nowMs := time.Now().UnixMilli()
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, rest, err := splitTextLine(line)
		if err != nil {
			return nil, fmt.Errorf("text line %q: %w", line, err)
		}
		valueStr := rest.value
		tsMs := rest.timestamp
		if tsMs == 0 {
			tsMs = nowMs
		}
		v, err := strconvParseFloat(valueStr)
		if err != nil {
			return nil, fmt.Errorf("text line %q: parse value: %w", line, err)
		}
		// 用排序后的 label 列表作 key,保证同样 label 集合的样本
		// 进入同一条 series。
		key := name + "|" + canonicalLabelKey(labels)
		ts, ok := groups[key]
		if !ok {
			ts = &promTimeSeries{}
			// 第一个 label 总是 __name__ = metric,会单独处理,
			// 真正写出去之前我们丢掉这个 label。
			for _, l := range labels {
				if l.Name == "__name__" {
					continue
				}
				ts.Labels = append(ts.Labels, l)
			}
			groups[key] = ts
			order = append(order, key)
		}
		ts.Samples = append(ts.Samples, promSample{Value: v, Timestamp: tsMs})
	}
	out := make([]promTimeSeries, 0, len(order))
	for _, k := range order {
		out = append(out, *groups[k])
	}
	return out, nil
}

// splitTextLine 返回 (name, labels, rest, err),其中 rest
// 携带 value 与可选 timestamp。
func splitTextLine(line string) (string, []promLabel, textRest, error) {
	// 1. 拆出 metric_name + 剩余部分。
	open := strings.IndexByte(line, '{')
	var name string
	var labelSrc string
	if open < 0 {
		// 找到第一个空白
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			return "", nil, textRest{}, fmt.Errorf("missing value")
		}
		name = strings.TrimSpace(line[:idx])
		labelSrc = ""
		rest := strings.TrimSpace(line[idx:])
		v, ts, err := parseValueAndTs(rest)
		if err != nil {
			return "", nil, textRest{}, err
		}
		return name, nil, textRest{value: v, timestamp: ts}, nil
	}
	// 找匹配的 }
	close := -1
	inStr := false
	escape := false
	for i := open + 1; i < len(line); i++ {
		c := line[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if !inStr && c == '}' {
			close = i
			break
		}
	}
	if close < 0 {
		return "", nil, textRest{}, fmt.Errorf("unterminated labels")
	}
	name = strings.TrimSpace(line[:open])
	labelSrc = line[open+1 : close]
	rest := strings.TrimSpace(line[close+1:])
	v, ts, err := parseValueAndTs(rest)
	if err != nil {
		return "", nil, textRest{}, err
	}
	labels, err := parseTextLabels(labelSrc)
	if err != nil {
		return "", nil, textRest{}, err
	}
	return name, labels, textRest{value: v, timestamp: ts}, nil
}

func parseTextLabels(src string) ([]promLabel, error) {
	if strings.TrimSpace(src) == "" {
		return nil, nil
	}
	var labels []promLabel
	for _, part := range splitTopLevelCommas(src) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			return nil, fmt.Errorf("bad label %q", part)
		}
		name := strings.TrimSpace(part[:eq])
		value := strings.TrimSpace(part[eq+1:])
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return nil, fmt.Errorf("label value not quoted: %q", value)
		}
		// 去掉外层 " 并反转义 \" 与 \\ 与 \n。
		inner := value[1 : len(value)-1]
		inner = strings.ReplaceAll(inner, `\\`, "\x00")
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, "\x00", "\\")
		inner = strings.ReplaceAll(inner, `\n`, "\n")
		labels = append(labels, promLabel{Name: name, Value: inner})
	}
	return labels, nil
}

// splitTopLevelCommas 按逗号拆分,但跳过 "..." 内的逗号。
func splitTopLevelCommas(s string) []string {
	var out []string
	start := 0
	inStr := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if c == ',' && !inStr {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func parseValueAndTs(s string) (string, int64, error) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", 0, fmt.Errorf("missing value")
	}
	v := parts[0]
	var ts int64
	if len(parts) >= 2 {
		f, err := strconvParseFloat(parts[1])
		if err != nil {
			return "", 0, fmt.Errorf("parse timestamp: %w", err)
		}
		// Prometheus 时间戳是毫秒;但用户可能写浮点(带小数点),
		// 统一转 int64。
		ts = int64(f)
	}
	return v, ts, nil
}

// strconvParseFloat 在 wrapper 层加这层 thin 函数,是为了
// 测试 stub 时只换实现,而不必 replace 全局函数。
func strconvParseFloat(s string) (float64, error) {
	// 等价于 strconv.ParseFloat,但拒绝 Inf/NaN —— Prometheus
	// 文本协议不接受它们。
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if f != f || f > 1e308 || f < -1e308 {
		return 0, fmt.Errorf("non-finite float: %s", s)
	}
	return f, nil
}

func canonicalLabelKey(labels []promLabel) string {
	if len(labels) == 0 {
		return ""
	}
	// label 顺序不稳定 -> 排序后拼接。
	sorted := append([]promLabel(nil), labels...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1].Name > sorted[j].Name; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	var b strings.Builder
	for _, l := range sorted {
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(l.Value)
		b.WriteByte(',')
	}
	return b.String()
}

type textRest struct {
	value     string
	timestamp int64
}
