package tracing

import (
	"encoding/hex"
	"testing"
)

func TestRandID(t *testing.T) {
	a := RandID(16)
	b := RandID(16)
	if a == b {
		t.Fatal("unique")
	}
	if len(a) != 32 {
		t.Fatalf("len: %d", len(a))
	}
}

func TestSpanBuilder_EndOK(t *testing.T) {
	st := NewTraceStore(10)
	sp := StartSpan(st, "http.get", KindServer).
		WithTenant("acme").
		Set("http.method", "GET").
		EndOK()
	if sp.Status != "ok" || sp.End.IsZero() {
		t.Fatalf("bad span: %+v", sp)
	}
	if sp.Duration() <= 0 {
		t.Fatal("duration should be positive")
	}
	got, ok := st.Get(sp.SpanID)
	if !ok || got.Name != "http.get" {
		t.Fatal("not stored")
	}
}

func TestSpanBuilder_EndError(t *testing.T) {
	st := NewTraceStore(10)
	sp := StartSpan(st, "db.query", KindClient).EndError("connection refused")
	if sp.Status != "error" || sp.StatusMsg != "connection refused" {
		t.Fatalf("%+v", sp)
	}
}

func TestSpanBuilder_WithTraceAndParent(t *testing.T) {
	st := NewTraceStore(10)
	tid := RandID(16)
	pid := RandID(8)
	sp := StartSpan(st, "inner", KindInternal).WithTrace(tid).WithParent(pid).EndOK()
	if sp.TraceID != tid || sp.ParentID != pid {
		t.Fatalf("%+v", sp)
	}
}

func TestSpanBuilder_WithTrace_EmptyIgnored(t *testing.T) {
	st := NewTraceStore(10)
	sp := StartSpan(st, "x", KindInternal).WithTrace("").EndOK()
	if sp.TraceID == "" {
		t.Fatal("empty WithTrace should keep auto id")
	}
}

func TestTraceStore_ByTrace(t *testing.T) {
	st := NewTraceStore(20)
	tid := RandID(16)
	for i := 0; i < 5; i++ {
		StartSpan(st, "op", KindInternal).WithTrace(tid).EndOK()
	}
	StartSpan(st, "other", KindInternal).EndOK()
	spans := st.ByTrace(tid)
	if len(spans) != 5 {
		t.Fatalf("expected 5, got %d", len(spans))
	}
	for i := 1; i < len(spans); i++ {
		if spans[i].Start.Before(spans[i-1].Start) {
			t.Fatal("not sorted")
		}
	}
}

func TestTraceStore_RingEviction(t *testing.T) {
	st := NewTraceStore(3)
	for i := 0; i < 5; i++ {
		StartSpan(st, "x", KindInternal).EndOK()
	}
	total := 0
	for _, sp := range st.List() {
		if sp != nil {
			total++
		}
	}
	if total != 3 {
		t.Fatalf("expected 3, got %d", total)
	}
	count, drop := st.Count()
	if count != 5 {
		t.Fatalf("count: %d", count)
	}
	if drop != 2 {
		t.Fatalf("expected 2 dropped, got %d", drop)
	}
}

func TestTraceStore_Add_Update(t *testing.T) {
	st := NewTraceStore(10)
	sp := StartSpan(st, "x", KindInternal).EndOK()
	sp.Attributes["extra"] = "1"
	st.Add(sp)
	got, _ := st.Get(sp.SpanID)
	if got.Attributes["extra"] != "1" {
		t.Fatal("update should preserve")
	}
}

func TestTraceStore_Add_NilOrEmpty(t *testing.T) {
	st := NewTraceStore(10)
	st.Add(nil)
	st.Add(&Span{})
	if st.Stats().Size != 0 {
		t.Fatal("nil/empty ignored")
	}
}

func TestTraceStore_GetMissing(t *testing.T) {
	st := NewTraceStore(10)
	if _, ok := st.Get("missing"); ok {
		t.Fatal("expected miss")
	}
}

func TestSpan_Validate(t *testing.T) {
	tidBytes := make([]byte, 16)
	for i := range tidBytes {
		tidBytes[i] = byte(i)
	}
	tid := hex.EncodeToString(tidBytes)
	sidBytes := make([]byte, 8)
	for i := range sidBytes {
		sidBytes[i] = byte(i)
	}
	sid := hex.EncodeToString(sidBytes)
	sp := &Span{TraceID: tid, SpanID: sid}
	if err := sp.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&Span{TraceID: "short", SpanID: sid}).Validate(); err == nil {
		t.Fatal("short trace id")
	}
	if err := (&Span{TraceID: tid, SpanID: "short"}).Validate(); err == nil {
		t.Fatal("short span id")
	}
	if err := (&Span{TraceID: tid, SpanID: sid, ParentID: "bad"}).Validate(); err == nil {
		t.Fatal("bad parent id")
	}
}

func TestSpan_Duration_Open(t *testing.T) {
	sp := &Span{}
	if sp.Duration() != 0 {
		t.Fatal("open span")
	}
}

func TestSampler_FullRate(t *testing.T) {
	s := NewSampler(1.0)
	for i := 0; i < 100; i++ {
		if !s.ShouldSample(RandID(16)) {
			t.Fatal("rate=1 should keep all")
		}
	}
}

func TestSampler_ZeroRate(t *testing.T) {
	s := NewSampler(0)
	for i := 0; i < 100; i++ {
		if s.ShouldSample(RandID(16)) {
			t.Fatal("rate=0 should drop all")
		}
	}
}

func TestSampler_HalfRate(t *testing.T) {
	s := NewSampler(0.5)
	kept := 0
	for i := 0; i < 10000; i++ {
		if s.ShouldSample(RandID(16)) {
			kept++
		}
	}
	if kept < 4000 || kept > 6000 {
		t.Fatalf("kept: %d", kept)
	}
}

func TestSampler_DeterministicForTrace(t *testing.T) {
	s := NewSampler(0.5)
	tid := RandID(16)
	r1 := s.ShouldSample(tid)
	r2 := s.ShouldSample(tid)
	if r1 != r2 {
		t.Fatal("same trace should give same answer")
	}
}

func TestSampler_SetRateClamps(t *testing.T) {
	s := NewSampler(0.5)
	s.SetRate(-1)
	if s.Rate() != 0 {
		t.Fatal("negative")
	}
	s.SetRate(2)
	if s.Rate() != 1 {
		t.Fatal("too big")
	}
}

func TestSampler_Stats(t *testing.T) {
	s := NewSampler(0.5)
	for i := 0; i < 100; i++ {
		s.ShouldSample(RandID(16))
	}
	st := s.Stats()
	if st.Count != 100 || st.Kept+st.Count-st.Kept != 100 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestClamp(t *testing.T) {
	if clamp(-1, 0, 1) != 0 {
		t.Fatal("neg")
	}
	if clamp(2, 0, 1) != 1 {
		t.Fatal("big")
	}
	if clamp(0.5, 0, 1) != 0.5 {
		t.Fatal("mid")
	}
}

func TestHashFraction(t *testing.T) {
	if hashFraction("hello") == hashFraction("world") {
		t.Fatal("should differ")
	}
	if v := hashFraction("hello"); v < 0 || v >= 1 {
		t.Fatalf("out of range: %v", v)
	}
}

func TestConcurrent_StoreSafe(t *testing.T) {
	st := NewTraceStore(100)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				StartSpan(st, "op", KindInternal).EndOK()
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
