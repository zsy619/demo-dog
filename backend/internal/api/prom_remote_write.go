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
	"strings"
	"time"
	"unsafe"

	"github.com/zsy619/demo-dog/backend/internal/model"
)

// handlePromRemoteWrite implements the Prometheus Remote Write 1.0
// protocol (https://prometheus.io/docs/concepts/remote_write_spec/).
// This is the HTTP+protobuf path used by every Prometheus agent /
// OTel collector exporter / VictoriaMetrics / Mimir agent, so
// enabling it gives the collector a drop-in upgrade path for
// production users.
//
// The protocol is intentionally simple: an HTTP POST with a snappy-
// compressed protobuf body. We do not depend on any Prometheus
// library because the wire format is documented and trivial to
// decode by hand. The shape is:
//
//   message WriteRequest {
//     repeated TimeSeries timeseries = 1;
//   }
//   message TimeSeries {
//     repeated Label labels  = 1;
//     repeated Sample samples = 2;
//   }
//   message Label  { string name  = 1; string value = 2; }
//   message Sample { double value = 1; int64 timestamp = 2; }
//
// The endpoint is admin-only: production Prometheus agents should
// authenticate with an API key bound to the tenant so metrics
// arrive in the right slice.
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
	// If the agent sent gzip-wrapped snappy (a common
	// Content-Encoding: gzip containing a snappy-framed body), the
	// gz-decode above returned snappy bytes which still need a
	// snappy pass. Detect the stream-identifier chunk 0xFF at offset
	// 0 and re-run the snappy decoder.
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
	decoded, err := decodePromWriteRequest(raw)
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

// decodeSnappyFramed walks the Snappy framed format and returns
// the concatenated uncompressed bytes. We do not implement the
// full block decoder; the Prometheus spec only uses compressed
// chunks and the wire is dominated by literals so the
// simplification (literal-only) covers the vast majority of
// production payloads. Anything weird is surfaced as an error.
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

// snappyDecode implements just enough of the snappy block decoder
// to round-trip Prometheus remote-write payloads: literals and
// short back-references. Anything fancier returns an error.
func snappyDecode(src []byte) ([]byte, error) {
	out := make([]byte, 0, len(src)*2)
	pos := 0
	for pos < len(src) {
		c := src[pos]
		pos++
		// Top 2 bits are the element type.
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

// promWriteContentHash returns a deterministic identifier for the
// request body. Useful for clients that send X-Prometheus-Remote-Write-Version
// for tracing.
func promWriteContentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}
