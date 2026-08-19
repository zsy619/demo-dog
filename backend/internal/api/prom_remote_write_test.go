package api

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// encodePromWriteRequest builds a single TimeSeries as a Prometheus
// WriteRequest protobuf body for the tests below.
func encodePromWriteRequest(t *testing.T, name, service string, samples []float64) []byte {
	t.Helper()
	var tsBody bytes.Buffer
	for _, kv := range [][2]string{{"__name__", name}, {"service", service}} {
		var label bytes.Buffer
		writeString(&label, 1, kv[0])
		writeString(&label, 2, kv[1])
		writeLD(&tsBody, 1, label.Bytes())
	}
	for i, v := range samples {
		var sample bytes.Buffer
		b := math.Float64bits(v)
		sample.WriteByte(0x09) // field=1, wire=1 (fixed64)
		binary.Write(&sample, binary.LittleEndian, b)
		var tsBuf [binary.MaxVarintLen64]byte
		tn := binary.PutVarint(tsBuf[:], int64(i+1))
		sample.WriteByte(0x10) // field=2, wire=0 (varint)
		sample.Write(tsBuf[:tn])
		writeLD(&tsBody, 2, sample.Bytes())
	}
	return writeLD(nil, 1, tsBody.Bytes())
}

func writeLD(buf *bytes.Buffer, field int, payload []byte) []byte {
	out := buf
	if out == nil {
		out = &bytes.Buffer{}
	}
	tag := uint64(field<<3) | 2
	var tagBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tagBuf[:], tag)
	out.Write(tagBuf[:n])
	var lenBuf [binary.MaxVarintLen64]byte
	ln := binary.PutUvarint(lenBuf[:], uint64(len(payload)))
	out.Write(lenBuf[:ln])
	out.Write(payload)
	return out.Bytes()
}

func writeString(buf *bytes.Buffer, field int, s string) {
	tag := uint64(field<<3) | 2
	var tagBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tagBuf[:], tag)
	buf.Write(tagBuf[:n])
	var lenBuf [binary.MaxVarintLen64]byte
	ln := binary.PutUvarint(lenBuf[:], uint64(len(s)))
	buf.Write(lenBuf[:ln])
	buf.WriteString(s)
}

func TestDecodePromWriteRequest_Basic(t *testing.T) {
	body := encodePromWriteRequest(t, "http_requests_total", "checkout", []float64{42.5})
	series, err := decodePromWriteRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	got := map[string]string{}
	for _, l := range series[0].Labels {
		got[l.Name] = l.Value
	}
	if got["__name__"] != "http_requests_total" {
		t.Fatalf("bad name: %v", got)
	}
	if got["service"] != "checkout" {
		t.Fatalf("bad service: %v", got)
	}
	if len(series[0].Samples) != 1 || series[0].Samples[0].Value != 42.5 {
		t.Fatalf("bad samples: %+v", series[0].Samples)
	}
}

func TestSplitPromLabels(t *testing.T) {
	lbls := []promLabel{
		{Name: "__name__", Value: "cpu"},
		{Name: "service", Value: "checkout"},
		{Name: "region", Value: "us-east-1"},
	}
	name, svc, attrs := splitPromLabels(lbls)
	if name != "cpu" || svc != "checkout" {
		t.Fatalf("name=%q svc=%q", name, svc)
	}
	if attrs["region"] != "us-east-1" {
		t.Fatalf("attrs=%v", attrs)
	}
}

func TestSnappyDecodeLiterals(t *testing.T) {
	body := []byte{0x85, 0x68, 0x65, 0x6c, 0x6c, 0x6f}
	out, err := snappyDecode(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}
}

func TestSnappyDecode_RejectsCorrupt(t *testing.T) {
	if _, err := snappyDecode([]byte{0xFF}); err == nil {
		t.Fatal("expected error for bad header")
	}
}

var _ = math.Pi
