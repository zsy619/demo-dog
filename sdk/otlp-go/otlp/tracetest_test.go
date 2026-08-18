package otlp

import (
	"context"
	"testing"
	"time"
)

func TestTracetestSDK(t *testing.T) {
	sdk, col := NewTestSDK(t)

	sdk.Log(context.Background(), SeverityInfo, "hello")
	sdk.Counter(context.Background(), "m", 1)
	_, end := sdk.Trace(context.Background(), "op")
	end(nil)

	if err := sdk.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}

	logs := col.Logs("test-service")
	if len(logs) != 1 {
		t.Fatalf("got %d logs", len(logs))
	}
	if logs[0].Body != "hello" {
		t.Errorf("body: %q", logs[0].Body)
	}

	mets := col.Metrics("test-service")
	if len(mets) != 1 {
		t.Fatalf("got %d metrics", len(mets))
	}

	spans := col.Spans("")
	if len(spans) != 1 {
		t.Fatalf("got %d spans", len(spans))
	}

	if col.CallCount() == 0 {
		t.Errorf("CallCount: %d", col.CallCount())
	}
}

func TestTracetestWaitForLogs(t *testing.T) {
	sdk, col := NewTestSDK(t)

	go func() {
		for i := 0; i < 5; i++ {
			sdk.Log(context.Background(), SeverityInfo, "msg")
			time.Sleep(2 * time.Millisecond)
		}
	}()

	logs := col.WaitForLogs(5, 500*time.Millisecond, "test-service")
	if len(logs) < 5 {
		t.Fatalf("got %d logs", len(logs))
	}
}

func TestTracetestReset(t *testing.T) {
	_, col := NewTestSDK(t)
	col.Export(nil, Request{Logs: []LogRecord{{Body: "x"}}})
	if col.CallCount() != 1 {
		t.Fatalf("calls: %d", col.CallCount())
	}
	col.Reset()
	if col.CallCount() != 0 || len(col.Requests()) != 0 {
		t.Fatalf("after reset: calls=%d reqs=%d", col.CallCount(), len(col.Requests()))
	}
}
