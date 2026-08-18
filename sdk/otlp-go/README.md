# otlp-go — DOG 采集器 Go SDK

纯 Go 实现的 SDK,用于上报 **JSON 简化版 OTLP** 三种信号(日志 / 指标 /
链路追踪),目标端是 [demo-dog](https://github.com/zsy619/demo-dog) 采集器。
SDK 是 `demo-dog` monorepo 的一部分,位于 `sdk/otlp-go/`,与后端摄入口
(`POST /api/ingest/otlp`) 的协议完全一致。

---

## 能力一览

| 维度 | 说明 |
|---|---|
| 依赖 | **零**(只用 Go 标准库) |
| 默认上报协议 | `application/json` + `/api/ingest/otlp`(简化 envelope) |
| 标准 OTel 协议 | `application/json+otlp` + `/api/ingest/otlp-json`(通过 `NewOTelExporter`) |
| Prometheus 协议 | `text/plain;version=0.0.4`,通过 `PrometheusCollector.Handler()` |
| 信号类型 | 日志 / 指标 / Span,加 Span links |
| 链路协议 | W3C Trace Context(`traceparent` / `tracestate`) |
| 采样 | `AlwaysOn/Off` / `TraceIDRatioBased` / `ParentBased` / 组合 / 自定义 |
| 并发模型 | 线程安全(单一全局缓冲 + 后台 worker goroutine) |
| 背压 | 批量大小受限 + 失败自动 requeue |
| SDK 自观 | `Stats()` 返回 12 个原子计数器(flush / drop / requeue / sample) |
| 错误路由 | `WithErrorHandler(fn)` 把内部错误接入业务的 logger |
| 测试支持 | `tracetest.InMemoryCollector` + `tracetest.NewTestSDK` |

---

## 安装

```bash
go get github.com/zsy619/demo-dog/sdk/otlp-go
```

本地直接用 monorepo 源码:

```go.mod
require github.com/zsy619/demo-dog/sdk/otlp-go v0.0.0
replace github.com/zsy619/demo-dog/sdk/otlp-go => ./sdk/otlp-go
```

---

## 5 分钟上手

```go
sdk, err := otlp.New("http://localhost:18080",
    otlp.WithService("checkout"),
    otlp.WithServiceVersion("v1.0.0"),
    otlp.WithDeploymentEnvironment("demo"),
    otlp.WithFlushInterval(2*time.Second),
    otlp.WithMaxBatch(500),
    otlp.WithSampler(otlp.NewTraceIDRatioBased(0.5)),    // 50% 采样
    otlp.WithAutoResource(true),                          // 注入 process.* / host.*
    otlp.WithErrorHandler(myLogger.LogError),             // 自定义错误路由
)
if err != nil { panic(err) }
defer sdk.Shutdown(context.Background())

ctx, end := sdk.Trace(context.Background(), "POST /checkout")
defer end(nil)

sdk.Log(ctx, otlp.SeverityInfo, "订单已下单",
    otlp.String("user_id", "u-42"),
    otlp.Int("items", 3))

sdk.Counter(ctx, "orders.placed", 1,
    otlp.String("channel", "web"))

start := time.Now()
charge()
sdk.Record(ctx, "POST /checkout/charge", start, nil)(
    otlp.String("user_id", "u-42"))
```

---

## SDK 选项(Option)表

| 选项 | 默认值 | 含义 |
|---|---|---|
| `WithService(name)` | — | 资源属性 `service.name`(**必填**) |
| `WithServiceVersion(v)` | — | 资源属性 `service.version` |
| `WithDeploymentEnvironment(env)` | — | 资源属性 `deployment.environment` |
| `WithHostName(host)` | — | 资源属性 `host.name` |
| `WithResourceAttrs(m)` | — | 合并额外资源属性 |
| `WithFlushInterval(d)` | `2s` | 后台 flush 周期 |
| `WithMaxBatch(n)` | `500` | 单次 flush 最多携带记录数 |
| `WithSampler(smp)` | `AlwaysOnSampler{}` | 采样器 |
| `WithErrorHandler(fn)` | `log.Printf` | SDK 内部错误路由 |
| `WithAutoResource(on)` | `false` | 自动注入 OTel 语义属性 |
| `WithExporter(e)` | 默认简化 JSON | 自定义 exporter,接受任意 `ExporterInterface` |
| `WithHTTPClient(c)` | 10s 超时 | 底层 `*http.Client` |
| `WithEndpoint(url)` | 推导 | 自定义上报 URL |

## Exporter 三选一

```go
// 1) 默认 — 简化 JSON envelope,/api/ingest/otlp
otlp.New(endpoint, ...)

// 2) 标准 OTel envelope,/api/ingest/otlp-json
//    (对接 vanilla OTel collector 时用)
otlp.New(endpoint, otlp.WithExporter(otlp.NewOTelExporter(endpoint)), ...)

// 3) Prometheus text,本地 /metrics scrape
otlp.New(endpoint, ...)
http.Handle("/metrics",
    otlp.NewPrometheusCollector(sdk).Handler())
```

## 指标 / 日志 / Trace API

### 日志

```go
sdk.Log(ctx, otlp.SeverityWarn, "缓存未命中",
    otlp.String("key", "user:42"),
    otlp.Int("retry", 2))

sdk.LogAttrs(ctx, otlp.LogRecord{
    Severity:   otlp.SeverityError,
    Body:       "上游超时",
    Attributes: map[string]string{"peer": "payments"},
})
```

严重级别字符串:`TRACE / DEBUG / INFO / WARN / ERROR / FATAL`。未识别字符串
归一化成 `INFO`。

### 指标

```go
sdk.Counter(ctx,   "orders.placed",     1,  otlp.String("channel", "web"))
sdk.Gauge(ctx,     "queue.depth",       42)
sdk.Histogram(ctx, "latency_ms",        78.4,
    otlp.String("path", "/checkout"))
```

### 链路追踪

```go
ctx, end := sdk.Trace(ctx, "POST /checkout")
defer end(nil)

start := time.Now()
work()
sdk.Record(ctx, "POST /checkout/charge", start, nil)(
    otlp.String("user_id", "u-42"))
```

## W3C Trace Context 跨服务传播

```go
prop := otlp.NewPropagator()

// 客户端:把 ctx 里的 trace 信息写到出站请求
req, _ := http.NewRequest("GET", "http://downstream/x", nil)
prop.InjectHTTPHeader(ctx, req)
http.DefaultClient.Do(req)

// 服务端:从入站请求里提取 trace 信息
tc := prop.ExtractHTTPHeader(r)
ctx := prop.WithTraceContext(r.Context(), tc)
_, end := sdk.Trace(ctx, "downstream.handle")
end(nil)
```

## 采样

```go
otlp.WithSampler(otlp.NewTraceIDRatioBased(0.1))     // 10% 确定性采样
otlp.WithSampler(otlp.NewParentBasedSampler(otlp.AlwaysOffSampler{}))
otlp.WithSampler(otlp.CompositeAnd(otlp.AlwaysOnSampler{}, myCustomSmp{}))

type Sampler interface {
    ShouldSample(c SampleContext) bool
    Description() string
}
```

被 sampler 拒绝的 trace 仍会 mint trace_id 并写到 ctx(以便传播),
只是在本地不会落 span。命中信息可以在 `Stats().SamplerSkipped` 看到。

## Span links

`SpanRecord.Links` 字段是 `[]SpanLink`,用于把当前 span 关联到另外
几个 trace(常见场景:异步消息消费、扇入 / 扇出)。后端简化 envelope 会
带 `links` 字段(后端当前会忽略,但会落到 span attributes)。OTel envelope
exporter 会按标准协议发到 `/api/ingest/otlp-json`。

```go
sdk.Record(ctx, "batch.process", start, nil)(
    otlp.String("batch_id", "b-99"))
// 然后单独 emit 一条 span(此处仅示意,SDK 当前不直接接收 links 参数)
```

## Prometheus 抓取

```go
http.Handle("/metrics",
    otlp.NewPrometheusCollector(sdk,
        otlp.WithPrometheusPrefix("myapp_")).Handler())
```

抓取端点会渲染当前 buffer 内全部 Counter / Gauge / Histogram。Histograms
作为 `<name>_sum` + `<name>_count` 暴露(单点样本,无 bucket)。
建议 scrape 前先 `sdk.ForceFlush(ctx)`。

## SDK 自观(Stats)

```go
st := sdk.Stats()
fmt.Printf("flush_calls=%d flush_errors=%d requeued_logs=%d sampled=%d\n",
    st.FlushCalls, st.FlushErrors, st.RequeuedLogs, st.SamplerSkipped)
```

字段:`LogsEmitted / MetricsEmitted / SpansEmitted / FlushCalls /
FlushErrors / RequeuedLogs / RequeuedMetrics / RequeuedSpans /
DroppedLogs / DroppedMetrics / DroppedSpans / SamplerSkipped`。

## 错误路由

```go
otlp.WithErrorHandler(func(err error) {
    sentry.CaptureException(err)
    // SDK 不再 log.Printf
})
```

## 单元测试

```go
sdk, col := otlp.NewTestSDK(t)
defer sdk.Shutdown(context.Background())

sdk.Log(ctx, otlp.SeverityInfo, "hello")
sdk.ForceFlush(ctx)

logs := col.Logs("test-service")
if len(logs) != 1 { t.Fatalf("got %d", len(logs)) }
```

`tracetest.InMemoryCollector` 提供 `Logs / Metrics / Spans / Requests /
WaitForLogs / Reset` 等方法,完全跑在内存里,零网络。

---

## 上报协议

SDK 发出的默认 envelope:

```json
{
  "resource_attrs": {
    "service.name": "checkout",
    "service.version": "v1.0.0",
    "telemetry.sdk.name": "otlp-go",
    "telemetry.sdk.language": "go",
    "telemetry.sdk.version": "0.1.0",
    "process.pid": "12345",
    "process.runtime.name": "go",
    "host.arch": "arm64",
    "os.type": "darwin"
  },
  "logs": [...],
  "metrics": [...],
  "spans": [...]
}
```

`Content-Type: application/json`,POST 到 `/api/ingest/otlp`。

如果要对接标准的 OTel collector,改用 `NewOTelExporter`(发到
`/api/ingest/otlp-json`,`Content-Type: application/json+otlp`)。

---

## 架构

```
┌──────────────────────────────────────────────────────────────────┐
│                        你的 Go 服务                               │
│                                                                   │
│   sdk.Log(…)   sdk.Counter(…)   sdk.Record(…)   sdk.Trace(…)     │
│        │            │              │               │             │
│        └────────────┴──────┬───────┴───────────────┘             │
│                           ▼                                       │
│                internal/buffer.Buffer (sync.Mutex)               │
│        logs []LogRecord   metrics []MetricPoint   spans []Span…   │
│                           │                                       │
│                  后台 goroutine                                  │
│                  tick = WithFlushInterval                         │
│                           │                                       │
│                           ▼                                       │
│             ExporterInterface.Export(ctx, Request)               │
│  ┌─ 简化 JSON (/api/ingest/otlp) ─────────────────────────┐    │
│  ├─ OTel envelope (/api/ingest/otlp-json) ───────────────┤    │
│  └─ Prometheus text (本地 /metrics scrape) ──────────────┘    │
└──────────────────────────────────────────────────────────────────┘
                            │
                            ▼
                demo-dog DOG-collector (Go)
                            │
                backend/internal/ingest/otlp.go
                            │
                            ▼
            backend/internal/batch/pool.go (worker pool)
                            │
                            ▼
            backend/internal/store/doris.go (in-memory Doris)
```

## 示例

完整列表见 [`examples/README.md`](./examples/README.md)。推荐先看
[`examples/demo-dog`](./examples/demo-dog),15 个示例覆盖了所有
主流使用场景:

```
quickstart     demo-dog       middleware      echo
gin            hertz          grpc-server     db-tracing
frontend-log-pipe              trace-propagation
kafka-consumer  worker-loop    otel-bridge
prometheus-exporter              sampler-debug
loadtest
```

---

## 协议约定 & 边界情况

| 场景 | SDK 行为 |
|---|---|
| collector 返回 5xx / network error | 整批 requeue 进 buffer,下次 flush 重试;`Stats.FlushErrors` +1 |
| `WithMaxBatch` 太小导致当前批次溢出 | 优先保留 logs,丢 metrics/spans;`Stats.DroppedLogs` 等 +N |
| `Severity` 传空字符串 | 默认 `INFO` |
| `Severity` 传 `WARNING` / `ERR` / `PANIC` | 归一化到 `WARN` / `ERROR` / `FATAL` |
| Sampler 拒绝 trace | trace_id 仍 mint + ctx 内仍注入,但 span 不落 buffer |
| `Record(ctx, ...)` 回调漏掉 attributes | 允许;attributes 是可选字段 |
| 同时存在两个 SDK 实例 | 支持;两者各有独立 buffer + worker |
| HTTP header 是空 / 畸形 traceparent | Extract 返回 nil,调用方需 fallback |
| `WithExporter(nil)` | 静默忽略,使用默认简化 exporter |

---

## 调试技巧

1. **SDK 是否真的在发** — 把 `WithFlushInterval` 设到 50ms,观察 collector 的请求日志
2. **SDK 是否在 requeue** — 看 `Stats().FlushErrors` 或 stderr 上 `otlp: export failed ...`
3. **batch trim 是否触发** — `WithMaxBatch(1)`,连续发 100 条 log,看 `DroppedLogs`
4. **采样比例是否正确** — 用 [`examples/sampler-debug`](./examples/sampler-debug)
5. **不想动真实 collector 跑测试** — 用 `tracetest.NewTestSDK(t)`

---

## 路线图

- [ ] 标准 `application/json+otlp` 协议(已完成,见 `NewOTelExporter`)
- [ ] Prometheus text format exporter(已完成,见 `PrometheusCollector`)
- [ ] OpenTelemetry trace_id 透传到 outgoing HTTP header(已完成,见 `Propagator`)
- [x] Sampler hook(已完成)
- [ ] Histogram bucket counters(目前是单点 sample)
- [ ] Process exporter(主动把 Go runtime metrics 上报成 metric)
- [ ] OTLP 压缩(gzip / snappy)

---

## 许可

MIT(与 `demo-dog` 主项目保持一致)。
