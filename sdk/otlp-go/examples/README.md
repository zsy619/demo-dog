# SDK 示例

每个示例都是独立、可运行的 `main` 程序。从项目根目录运行:

```bash
git clone https://github.com/zsy619/demo-dog.git
cd demo-dog/sdk/otlp-go
go run ./examples/<name>
```

> 所有示例默认连接 `http://localhost:18080`。如需指向远端 collector,
> 用环境变量 `DOG_ENDPOINT=http://host:port` 覆盖。

> 大多数示例只依赖 SDK 本身(零三方依赖)。`hertz` 例外,自带 `go.mod`,
> 通过 `replace` 指向主 SDK,因此运行前需要单独 `cd examples/hertz && go mod tidy`。

## 示例索引

### 入门

| 示例 | 端口 | 场景 | 一句话说明 |
|---|---|---|---|
| [`quickstart`](./quickstart) | — | 单文件 hello-world | 5 条 log + 5 个 counter + 1 个 trace + 1 个子 span |
| [`demo-dog`](./demo-dog) | — | 综合展示 | 推荐作为文档首段:Counter/Gauge/Histogram/Trace/Record/Log 一次性出场 |

### HTTP 服务埋点

| 示例 | 端口 | 框架 | 说明 |
|---|---|---|---|
| [`middleware`](./middleware) | `:8081` | stdlib `net/http` | 把每个请求包成 trace,带状态码 → severity |
| [`echo`](./echo) | `:8082` | Echo | stdlib 移植 + Echo 原生版注释 |
| [`gin`](./gin) | `:8087` | Gin | stdlib 移植 + Gin 原生版注释 + `/metrics` |
| [`hertz`](./hertz) | `:8088` | Hertz (CloudWeGo) | 真实 Hertz middleware + `/metrics` (自带 hertz 依赖) |
| [`frontend-log-pipe`](./frontend-log-pipe) | `:8083` | 浏览器→Go 反代 | React SPA 通过 POST 把 telemetry 反代到 SDK |
| [`trace-propagation`](./trace-propagation) | `:8084`/`:8085` | 跨服务 W3C | 同进程两个服务演示 traceparent 透传 |

### 数据层 / 消息层

| 示例 | 说明 |
|---|---|
| [`db-tracing`](./db-tracing) | `database/sql` 包装,给 `*sql.DB` 加 span + counter + histogram |
| [`grpc-server`](./grpc-server) | `net/rpc` 风格 RPC 服务,每调用一个 trace |
| [`kafka-consumer`](./kafka-consumer) | Kafka 风格消费者,每条消息一个 trace + 50% 采样演示 |

### 后台工作

| 示例 | 说明 |
|---|---|
| [`worker-loop`](./worker-loop) | 周期 tick:1 trace + 3 子 span + 1 counter + 1 gauge |

### 协议互操作

| 示例 | 说明 |
|---|---|
| [`otel-bridge`](./otel-bridge) | 把 OTel envelope 数据通过 SDK 转给 collector |
| [`prometheus-exporter`](./prometheus-exporter) | 同时暴露 `/metrics` 端点 + 推给 collector |

### 调试 / 测试

| 示例 | 说明 |
|---|---|
| [`sampler-debug`](./sampler-debug) | 1000 次 trace,展示 sampler 命中比 + SDK 内部 stats |
| [`loadtest`](./loadtest) | 多 producer 灌 50k log / 50k metric / 5k span,验 trim 路径 |

> 关于单元测试的 SDK 用户请用 [`../otlp/tracetest.go`](../otlp/tracetest.go) 中的
> `InMemoryCollector` + `NewTestSDK`,无需再写一个示例。

## 推荐学习顺序

1. `quickstart` — 先跑通,确认 collector 在工作
2. `demo-dog` — 一次看完全部 API 形态
3. `middleware` — 把 SDK 接进 HTTP 服务(最常见场景)
4. `db-tracing` — 给数据库加埋点
5. `worker-loop` — 给后台 worker 加埋点
6. `kafka-consumer` — 给消息消费者加埋点(同时演示 sampler)
7. `trace-propagation` — 看 trace 怎么跨服务传播
8. `otel-bridge` — 看 SDK 怎么讲 OTel 协议
9. `prometheus-exporter` — 同时支持 Prometheus 抓取 + DOG 上报
10. `sampler-debug` — 调采样率
11. `loadtest` — 压测 SDK

## 验证 SDK 是否正常工作的最快办法

```bash
# 1) 启 demo-dog 后端 + 前端
./scripts/dev.sh up

# 2) 启任意示例
go run ./examples/demo-dog

# 3) 看数据
curl -s http://localhost:18080/api/services | python3 -m json.tool
curl -s "http://localhost:18080/api/services/demo-dog-comprehensive/logs?limit=5" | python3 -m json.tool
```

返回的 JSON 里能看到你刚发出的 `logs_count / metrics_count / spans_count`
递增,就说明 SDK + collector + 后端 + 前端的整条链路是通的。

## 完整 SDK API 文档

见 [`../README.md`](../README.md)。
