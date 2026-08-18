// kafka-consumer -- demonstrates wrapping a message-driven worker with
// the SDK. Each message becomes a fresh trace; processing latency and
// success/failure are recorded as a histogram and counter.
//
// This example uses an in-memory fake consumer so the demo is
// self-contained. To use with a real broker (confluent-kafka-go,
// sarama, segmentio/kafka-go), replace FakeConsumer with the real
// consumer; the wrapper code does not change.
//
//   go run ./examples/kafka-consumer
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

// Message is the canonical Kafka-style envelope we process.
type Message struct {
	Topic     string
	Partition int
	Offset    int64
	Key       string
	Value     []byte
}

// Consumer is the interface a real Kafka client satisfies.
type Consumer interface {
	Poll(ctx context.Context) (*Message, error)
	Commit(ctx context.Context, m *Message) error
	Close() error
}

// FakeConsumer produces a deterministic stream of messages. Good enough
// for a demo: see kafka.Consumer above for the real interface.
type FakeConsumer struct {
	topic  string
	cursor int64
}

func NewFakeConsumer(topic string) *FakeConsumer { return &FakeConsumer{topic: topic} }

func (c *FakeConsumer) Poll(ctx context.Context) (*Message, error) {
	c.cursor++
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(50+rand.Intn(150)) * time.Millisecond):
	}
	return &Message{
		Topic:     c.topic,
		Partition: 0,
		Offset:    c.cursor,
		Key:       fmt.Sprintf("k-%d", c.cursor),
		Value:     []byte(fmt.Sprintf("payload-%d", c.cursor)),
	}, nil
}

func (c *FakeConsumer) Commit(_ context.Context, _ *Message) error { return nil }
func (c *FakeConsumer) Close() error                                { return nil }

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}
	sdk, err := otlp.New(endpoint,
		otlp.WithService("kafka-consumer-demo"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(2*time.Second),
		// T1.4: keep only 50% of traces to demonstrate sampling.
		otlp.WithSampler(otlp.NewTraceIDRatioBased(0.5)),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	c := NewFakeConsumer("orders")
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	processed := runConsumer(ctx, sdk, c)
	fmt.Printf("processed %d messages\n", processed)
}

func runConsumer(ctx context.Context, sdk *otlp.SDK, c Consumer) int {
	var wg sync.WaitGroup
	var processed int64
	done := make(chan struct{})
	go func() {
		for {
			msg, err := c.Poll(ctx)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					close(done)
					return
				}
				log.Printf("poll err: %v", err)
				continue
			}
			wg.Add(1)
			go func(m *Message) {
				defer wg.Done()
				handle(ctx, sdk, c, m)
				// atomic.AddInt64 would be nicer, but simple count here is fine
				processed++
			}(msg)
		}
	}()
	<-done
	wg.Wait()
	return int(processed)
}

func handle(ctx context.Context, sdk *otlp.SDK, c Consumer, m *Message) {
	// 1) Trace: each message is a root trace.
	ctx, end := sdk.Trace(ctx, "kafka.handle")
	start := time.Now()

	// 2) Counter for throughput.
	sdk.Counter(ctx, "kafka.messages", 1,
		otlp.String("topic", m.Topic),
		otlp.Int("partition", int64(m.Partition)),
	)

	// 3) Decode payload as JSON {"kind":...} for the demo.
	var payload struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(m.Value, &payload); err != nil {
		sdk.Log(ctx, otlp.SeverityWarn, "bad payload",
			otlp.String("error", err.Error()),
			otlp.Int("offset", m.Offset),
		)
		end(err)
		return
	}

	// 4) Sub-task spans.
	subStart := time.Now()
	time.Sleep(time.Duration(10+rand.Intn(40)) * time.Millisecond)
	sdk.Record(ctx, "kafka.validate", subStart, nil)(
		otlp.String("kind", payload.Kind),
	)

	subStart = time.Now()
	time.Sleep(time.Duration(15+rand.Intn(60)) * time.Millisecond)
	sdk.Record(ctx, "kafka.process", subStart, nil)()

	// 5) Histogram for end-to-end latency.
	dur := time.Since(start).Milliseconds()
	sdk.Histogram(ctx, "kafka.duration_ms", float64(dur),
		otlp.String("topic", m.Topic),
	)

	if err := c.Commit(ctx, m); err != nil {
		sdk.Log(ctx, otlp.SeverityError, "commit failed",
			otlp.String("error", err.Error()),
		)
		end(err)
		return
	}
	end(nil)
}
