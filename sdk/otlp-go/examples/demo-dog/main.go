// demo-dog — 综合性单文件示例。
//
// 这个示例一次展示 SDK 的所有核心能力:资源属性、Counter/Gauge/Histogram
// 三种 metric、Trace + Record 嵌套链路、Context 传播 trace_id、
// ForceFlush、Shutdown 的语义区别。适合放进 README 作为推荐入口。
//
//   go run ./examples/demo-dog
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	// 1) 构造 SDK,显式覆盖所有常用 option。
	sdk, err := otlp.New(endpoint,
		otlp.WithService("demo-dog-comprehensive"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithDeploymentEnvironment("dev"),
		otlp.WithHostName("laptop-01"),
		otlp.WithFlushInterval(500*time.Millisecond),
		otlp.WithMaxBatch(100),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	ctx := context.Background()

	// 2) 在 SDK 启动后立即上报一批“服务自身”指标——队列深度、版本号。
	sdk.Gauge(ctx, "process.queue_depth", 0,
		otlp.String("queue", "ingest"),
	)
	sdk.Counter(ctx, "process.started", 1,
		otlp.String("service.version", "0.1.0"),
	)

	// 3) Trace + Record:一个根 trace 下三个子任务,其中一个故意失败。
	rootCtx, endRoot := sdk.Trace(ctx, "checkout.run")
	defer endRoot(nil)

	if err := step(rootCtx, sdk, "fetch_user", 40, nil); err != nil {
		endRoot(err)
		log.Printf("checkout failed: %v", err)
		return
	}
	if err := step(rootCtx, sdk, "charge_card", 120, nil); err != nil {
		endRoot(err)
		log.Printf("checkout failed: %v", err)
		return
	}
	// 故意失败一个,观察它在 DOG UI 上 span.status=error 的样子。
	errBoom := step(rootCtx, sdk, "send_receipt", 80,
		errors.New("smtp connection refused"))

	// 4) 业务层 metric:counter + histogram。
	sdk.Counter(rootCtx, "checkout.attempts", 1,
		otlp.String("result", statusOf(errBoom)),
	)
	sdk.Histogram(rootCtx, "checkout.latency_ms", 240,
		otlp.String("result", statusOf(errBoom)),
	)

	// 5) 结构化日志:全栈上下文 + 严重级别。
	sdk.Log(rootCtx, otlp.SeverityInfo, "checkout flow finished",
		otlp.String("step.send_receipt", statusOf(errBoom)),
		otlp.Int("items", 3),
		otlp.Float("amount", 99.5),
	)

	// 6) ForceFlush:任务在关键节点主动刷盘,不等后台 ticker。
	if err := sdk.ForceFlush(ctx); err != nil {
		log.Printf("force flush: %v", err)
	}

	fmt.Println("emitted: 1 trace (3 spans), 3 counters, 2 gauges, 1 histogram, 1 log")
}

// step 是嵌套 span 的最小演示。返回的 error 会被 sdk.Record 自动映射成
// span.status=error。
func step(parent context.Context, sdk *otlp.SDK, name string, sleepMs int, wantErr error) error {
	start := time.Now()
	time.Sleep(time.Duration(sleepMs+rand.Intn(20)) * time.Millisecond)
	sdk.Record(parent, "step."+name, start, wantErr)(
		otlp.String("step.name", name),
	)
	if wantErr != nil {
		sdk.Log(parent, otlp.SeverityError,
			"step "+name+" failed",
			otlp.String("error", wantErr.Error()),
		)
	}
	return wantErr
}

func statusOf(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}
