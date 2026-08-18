# demo-dog 项目评测报告(企业级差距分析)

> 评估日期:基于当前仓库快照。范围:`/Volumes/E/JYW/创意项目/demo-dog/`,SDK 以 `sdk/otlp-go` 为准。
> 项目定位:自述为"教学/演示"用的全栈可观测性平台(Doris+OTel+Grafana 复合词)。

---

## A. SDK 能力差距(企业可用性)

### 1. 语言覆盖

- ❌ **只有 Go SDK**。`sdk/` 目录下仅有 `otlp-go/`,没有 Java/Node/Python/.NET/PHP/Ruby SDK。
  - 业务现状几乎只有 Go 后端可用。
  - 当公司引入 Java/Node/Python/.NET/PHP/Ruby 等技术栈后,只能依赖各语言的 OpenTelemetry 上游 SDK,把数据送进 demo-dog 的 `/api/ingest/otlp` 简化 JSON 端点 —— 但简化 envelope 与官方 OTLP/OTel 标准 envelope 在 schema 上有差异,测试矩阵将成倍放大。
- ⚠️ README 与 SDK README 都没有"目标用户语言"的说明,接入方无从知晓"现在支持什么语言,自己团队缺什么"。
- ❌ 没有按标准 OTel 注册中心(release 元数据、Maven Central / PyPI / npm 等)发布计划,社区贡献也无从着手。

### 2. 协议支持

- ✅ 简化版 JSON envelope:`POST /api/ingest/otlp`,字段 `{resource_attrs, logs, metrics, spans}`(见 `sdk.go::NewExporter`、`types.go::Request`)。
- ✅ 标准 OTel JSON envelope:`POST /api/ingest/otlp-json`,通过 `otlp.NewOTelExporter` 调用,Content-Type `application/json+otlp`(`envelope.go::EncodeOTLPEnvelope`)。
- ✅ Prometheus 文本格式(`text/plain; version=0.0.4`),通过 `PrometheusCollector.Handler()` 暴露本地 `/metrics`(`prometheus.go`)。
- ❌ **OTLP/gRPC** 未实现。仓库中 `grep "grpc|thrift|zipkin|jaeger"` 在 `*.go` 里 0 命中(只命中了示例中的 `grpc-server/main.go` 名称,内容是 RPC 模拟,不是 OTLP gRPC)。
- ❌ **Zipkin v2 JSON/Thrift** 接收器未实现。
- ❌ **Jaeger Thrift** 接收器未实现。
- ❌ **Prometheus Remote Write** 未实现 —— 当前只有本地 scrape,无法主动推送远端。
- ❌ **OTLP/Protobuf**(`application/x-protobuf`)编码未实现,只能 JSON。Schema 大、跨语言兼容时 JSON 的体积/精度问题会爆出来(见 README 路线图"增加 OpenTelemetry Protobuf 导出器"仍是 TODO)。
- ⚠️ 简化 envelope 与标准 OTel envelope 之间需要 SDK 调用方通过 `WithExporter` 手动选择,SDK 没有自动协商或 controller 探测机制。

### 3. 自动埋点(Agent 模式 / OTel API 兼容)

- ❌ **无任何 auto-instrumentation / agent 模式**。`sdk/otlp-go` 全是 manual API:`sdk.Log` / `sdk.Counter` / `sdk.Gauge` / `sdk.Histogram` / `sdk.Trace` / `sdk.Record`。
- ❌ **不兼容 OpenTelemetry API**(`go.opentelemetry.io/otel`)。**没有 `otel.Tracer`/`otel.Meter`/`otel.Logger` 桥接层**。
  - `sdk/otlp-go/go.mod` 仅有 `module github.com/zsy619/demo-dog/sdk/otlp-go` 与 `go 1.22`,**零依赖** —— 同时也意味着 OTel API/SDK 主线协议面无法对接。
  - 示例 `otel-bridge/main.go` 名为 "bridge",但内容只是用 `OTelExporter` 序列化,不是真正暴露 OTel Tracer/Meter API。
- ⚠️ 没有标准 OTel HTTP semantic-conventions helper。`semconv.go` 仅有 process/host 维度的 auto-resource,应用层 HTTP/DB/MQ 调用方需要手写属性名。
- ❌ 没有运行时 hook(eBPF / JVMTI / Monkey Patch)型的零侵入 agent。

### 4. 资源检测(Resource Detection)

- ✅ 仅基础进程属性,在 `WithAutoResource(true)` 时注入:
  - `process.pid`、`process.runtime.name`、`process.runtime.version`、`process.executable.name`
  - `host.arch`、`host.type`、`os.type`
  - `telemetry.sdk.name=otlp-go`、`telemetry.sdk.language=go`、`telemetry.sdk.version=0.1.0`
- ❌ **K8s pod labels / annotations / namespace / serviceAccount** 未探测。`grep "k8s|kubernetes" *.go` 在 SDK 中 0 命中。
- ❌ **Cloud Provider metadata**(AWS EC2 / ECS / EKS / GCP / Azure)未实现,无 `cloud.provider`、`cloud.platform`、`cloud.region` 等 OTel standard resource attribute。
- ❌ **Container ID / hostname FQDN** 未注入(`WithHostName(host)` 只取字符串,不探测)。
- ❌ 不感知 deployment 模式(`deployment.environment` 仅由 `WithDeploymentEnvironment` 手动传入)。
- ⚠️ `applyAutoResource` 默认 **关**,需用户主动开启,文档没有强烈推荐。

### 5. 采样智能

- ✅ 内置 5 个 Sampler,见 `sampler.go`:
  - `AlwaysOnSampler`(默认)
  - `AlwaysOffSampler`
  - `TraceIDRatioBased(ratio)`(基于 trace_id 哈希的确定性分桶,**head-based**)
  - `ParentBasedSampler`(跟随上游的采样决定,root fallback 到 `root` sampler)
  - `CompositeAnd / CompositeOr`
- ❌ **无 tail-based sampling**(在 collector 看完 trace 后再判定保留/丢弃)。需要 RBE(remote probabilistic/rule based)、latency-aware、error-aware 等组合。
- ❌ **无 error / status-based sampler**(`sampler-go` 的 `StatusCodeErrorSampler` 等效物缺失)。
- ❌ **无 latency-based sampler**(长于 P95 的请求保留等)。
- ❌ **无 remote / dynamic sampler**(通过 `/api/config` 从后端拉远程概率)。
- ⚠️ `Stats().SamplerSkipped` 只有总命中数,缺分 sampler / 分服务的命中分布。
- ⚠️ Sample 决策只看 `TraceID/SpanID/Name/HasParent`,不能携带业务属性(如 `http.route`、`error.type`),无法做规则化采样。

### 6. 批量与背压(Collector 宕机处理)

- ✅ 后台 goroutine 单 worker 周期性 flush,默认 `WithFlushInterval=2s`、`WithMaxBatch=500`(`sdk.go::run`)。
- ✅ 整批失败 requeue 进 buffer,下次 flush 重试(`sdk.go::requeue`)。`Stats.FlushErrors`、`Stats.RequeuedLogs/Metrics/Spans` 可观测。
- ✅ `WithHTTPClient` 允许自定义 client,但默认 10s 超时(`exporter.go::NewExporter`)。
- ⚠️ **buffer 没有真正的容量上限** —— `internal/buffer/buffer.go` 中 `b.logs = append(b.logs, l)` 是无限增长的切片,只有 `maxBatch` 在 flush 时做 trim(`trimBufferBatch`)。
  - collector 长时间宕机 → 内存会无限增长,直到被 OS OOM。
  - trim 逻辑保 logs 优先丢 spans/metrics,但仍在内存里堆积。
- ⚠️ 重试策略简单 requeue,**没有指数退避 / 最大重试次数 / 死信**。重复失败的批量会无限循环阻塞后续流量。
- ❌ 没有 circuit breaker。连续失败时不会暂停 push,反而会占用 buffer。
- ❌ 没有节流到 SDK 调用方。SDK 入队无上限。
- ❌ 没有持久化(spool 到 file)。进程重启 = 丢数据。

### 7. 指标(Metrics)的丰富度

- ✅ Counter / Gauge / Histogram 三类(`types.go::MetricType`)在 SDK 层暴露。
- ✅ Histogram 作为 OTel Histogram DP 上报(`envelope.go::OTelHistogramDP` 字段 `Count`、`Sum`、`Min`、`Max`、`Mean`)。
- ⚠️ **SDK Histogram 是单点 sample**,没有真正的 bucket 累积。`Histogram(ctx, name, value, kvs...)` 只把 `value` 当一个观测点 push 出去,不像 Prometheus client 那样维护 bucket counter。
- ⚠️ Prometheus exporter 暴露 `<name>_sum` + `<name>_count`,**不带任何 bucket/le 行**,见 `prometheus.go::writeMetricLine`(KIND 为 histogram 时只写两行,PROM 抓取端无 buckets 可算 p95)。README 也明确承认了。
- ❌ **无 Summary metric 类型**,只有三种 MetricType。
- ❌ **无 Exemplars**(链接到 trace_id 的样本点)。
- ❌ **无 OTLP exponential histogram / native histogram**(2024+ OTel 标准 spec 已引入)。
- ❌ **无 View / Aggregation 配置**。SDK 内部只有一个简单聚合视角。
- ❌ **无 metric stream reuse / 缓存**。同类同 label 每次都生成新 DP,backend 不能去重/降采样。
- ❌ 无 aggregation temporality 配置(delta vs cumulative),固定写死(需要查 envelope.go 具体数值)。
- ⚠️ PrometheusExporter 不带 `# HELP` 行以外的实际注释行(仅有 `# HELP <name> exported by otlp-go`),缺 UNITY 注释(`# UNIT`)。

### 8. 日志功能(Log Features)

- ✅ 6 个严重级别 enum:`Severity{Trace, Debug, Info, Warn, Error, Fatal}`(`types.go`)。未知值归一化 INFO(`transform.NormalizeSeverity`)。
- ✅ 结构化属性 `Attributes` 是 `map[string]string`(`types.go::LogRecord`)。
- ✅ `LogAttrs` 接收预制 `LogRecord`。
- ❌ **无 log appender 体系**。SDK 不像 log4j/logback/zerolog 那样可换 backend。
- ❌ **无日志级别运行时开关** —— Sampler/FlushInterval 等都是构造期 Option,运行期不可改。
- ❌ **无 BatchProcessorf 之分流策略**(比如 ERROR 自动落本地文件,INFO 走 collector)。
- ❌ **LogRecord.Body 强类型 string**,只支持结构化 attributes,**不支持原始 `any` body**(OTel 标准是 AnyValue,可以嵌套 map/array)。
- ❌ **不带 trace 自动注入方法** —— 必须先调用 `sdk.Trace` 把 trace_id 写进 ctx,然后 `sdk.Log` 才会自动带上(`LogAttrs` 自己读 ctx);普通调用者容易遗忘。
- ❌ **无 TraceFlags / dropped-attributes-count 等 OTel 高级字段**,与标准 OTLP log record 不完全对齐。

### 9. 配置(Configuration)

- ✅ 全部通过函数式 Option(`SDKOption` / `ExporterOption` / `OTelExporterOption` / `PrometheusOption`)配置,20+ 选项(`sdk.go` 全表)。
- ❌ **完全无环境变量读取**。除 examples 里 `DOG_ENDPOINT` 之外,SDK 自身不消费任何 `OTEL_*` 环境变量(标准 OTel 规范都约定从 env 读 `OTEL_SERVICE_NAME` 等)。
- ❌ **无配置文件读取**(YAML / TOML / JSON)。
- ❌ **无远程配置中心** 接入(无 `WithRemoteConfig` / Opamp 客户端)。
- ❌ **无 auto-discovery**(本地 4317/4318 探活 / KMS)。
- ⚠️ 端点合并靠字符串拼接 `joinEndpoint`(base+path),容易出现双斜杠或漏前缀。
- ❌ **无热更新 / 动态 reload**:FlushInterval、Sampler、Endpoint 一旦构造完都是常量。

### 10. 安全(Security)

- ❌ **无 mTLS / 客户端证书支持**。HTTP client 可以由用户传入,可以自己实现 TLS,但 SDK 不提供简便封装。
- ❌ **无 auth 头注入**。没有 `WithBearerToken`、`WithAPIKey`、`WithHeader` 等快捷方式。
  - 用户必须 `WithHTTPClient(myClient)` 并自行加 roundtripper。
- ❌ **无 PII / 敏感字段脱敏**。`Attributes` 透传,SDK 不审计 key/value。
- ❌ **后端 collector 自身无身份验证**。`/api/ingest/otlp` 是开放的(`backend/internal/api/handlers.go` 与 nginx/Caddy 配置没有任何 `auth_basic` / bearer / IP allowlist)。
- ❌ 无 audit log、无 rate limit。
- ⚠️ 后端进程以 dog(uid 10001)非 root 运行(`Dockerfile` 第 44 行),这是仅有的最小硬化措施。
- ⚠️ Caddy `Caddyfile` 启用 `admin off`、`auto_https off`(生产可以省略),HTTPS 仅在 `Caddyfile.https` 提供且需用户自行替换域名。

---

## B. 工程 / 部署 / 治理差距

### 1. 容器化与编排

- ✅ `Dockerfile`:多阶段(`backend-builder` / `backend` / `frontend-builder` / `frontend` / `all`),静态二进制 CGO=0,Alpine 镜像。
  - 后端运行 healthcheck、USER dog、EXPOSE 8080。
- ✅ `docker-compose.yml`:backend(Go) + frontend(nginx)双服务,带 healthcheck 与 `depends_on: service_healthy`。
- ✅ `docker-compose.caddy.yml`:同样的拓扑换成 Caddy。
- ✅ `deploy/Dockerfile.caddy` 镜像 + 三个 Caddyfile(`Caddyfile`、`Caddyfile.fullstack`、`Caddyfile.https`)。
- ✅ nginx 与 Caddy 两套等价配置,带反向代理、SPA fallback、WebSocket upgrade、缓存控制(`deploy/README.md` 还有完整指令对照表)。
- ✅ `deploy/start-all.sh` 用于 `--target all` 镜像的进程管理。
- ❌ **无 Kubernetes manifest**(无 `*.yaml` / `Deployment` / `StatefulSet` / `Service` / `Ingress`,`ls -la k8s manifests helm charts` 全部为空)。
- ❌ **无 Helm Chart**。
- ❌ **无 Kustomize / Jsonnet**。
- ❌ **无 docker swarm / Nomad / ECS / GKE Deployments**。
- ⚠️ README 路线图明确标注 "提供 Kubernetes 部署的 Helm Chart" 是 TODO。
- ⚠️ Docker Compose 只在单机/演示场景,不写 volume、不写 secret、外部化配置全靠 `environment:` 直填。

### 2. CI/CD 工作流

- ❌ **完全没有 CI 配置文件**:
  - `ls -la .github` 不存在
  - `ls -la .gitlab-ci.yml .circleci Jenkinsfile` 全不存在
- ❌ 无 GitHub Actions / GitLab CI / Tekton / Drone。
- ❌ 无 release 流水线(无 goreleaser、goreleaser-cross、nfpm 等,只有 `Makefile` 手动 `build`)。
- ❌ 无镜像签名 / cosign / SLSA provenance。
- ⚠️ `scripts/smoke.sh` 9 项检查可以作为 CI 缺失时的"半自动"冒烟入口,但需要手动 `make smoke-up` 启后端。

### 3. 架构决策记录(ADR)

- ❌ **无 `docs/adr/`、无 `adr/`、无 `ARCHITECTURE.md` 文件**。
- ⚠️ `README.md` 内嵌了非常详尽的 mermaid 架构图、时序图、数据流图,可以算"非正式"架构描述,但**没有 ADR** 解释"为什么选 Doris 内存版"、"为什么 base 在 in-memory 而不是持久化"、"为什么采样只在 SDK 不在 collector"、"为什么 envelope 简化而不是完整 OTLP"。
- 读者只能从代码反推决策。

### 4. 许可证(License)

- ⚠️ **README 末尾声明 "MIT"**,但 **仓库根目录没有 `LICENSE` 文件**。
  - 开源合规上,没有 LICENSE 文件意味着下游无法以权威方式使用 MIT 条款。
  - SDK README 也声明 MIT,同样没有 LICENSE。
- ⚠️ 没有 `NOTICE`、`COPYING`、`THIRD_PARTY` 等相关法律文件。
- ❌ 无法判断依赖合规性 —— SDK 零依赖,但**前端** `package.json` 未审阅,依赖许可证情况不透明。

### 5. 贡献指南(Contributing)

- ❌ **无 `CONTRIBUTING.md`**。
- ❌ 无 `.github/ISSUE_TEMPLATE/`、`PULL_REQUEST_TEMPLATE`。
- ❌ 无 `CODE_OF_CONDUCT` / `SECURITY.md`。
- ❌ 无 `OWNERS` / `MAINTAINERS` / governance 文件。
- 新贡献者无法知道分支策略、commit 规范、code style 偏好(只用 `gofmt -w`,无 lint 工具如 golangci-lint 配置)。

### 6. 变更日志(Changelog)

- ❌ **无 `CHANGELOG.md`**。
- ❌ 无 release tag(`Makefile` 提到 `VERSION ?= 0.1.0`,且 SDK 的 `Version` 常量 = `0.1.0`,说明版本号是手维护)。
- ⚠️ 后端启动横幅打印 `DOG-collector v0.1.0`,SDK 的 `Version` 也能在 binary 里覆盖(ldflags),但缺 CHANGELOG,版本之间变更无从对照。

### 7. 版本化与 API 稳定性承诺

- ⚠️ 顶层 `Makefile` 默写 `VERSION ?= 0.1.0`,Docker `image: demo-dog-backend:0.1.0`。
- ❌ **没有 SemVer 约定** —— `v0.1.0` 是 pre-1.0,可以随手 break。
- ❌ **没有 "stable API promise"** —— SDK README、Go module path、ExporterInterface 都是公开发布面,任何字段变更都没有 deprecation 流程。
- ❌ 没有 go.mod 的 `require` 之外,在 SDK 目录下没有 `CODEOWNERS`,没有 `Stable` 注解(Gradle-style),没有 Go 兼容标记。
- ⚠️ 内部包用 `internal/buffer` 算一个正确的隐藏内部细节的做法(✓),但导出面(`otlp.Request`、`otlp.LogRecord` 等)同样为公开 API 且没有 compatibility matrix。

### 8. 多区域 / HA / 灾备

- ❌ **后端是有状态 in-memory**(`backend/internal/store/doris.go`)—— ring buffer + 物化视图全部在进程内存里。
  - 没有持久化(无 BoltDB/LevelDB/WAL),没有 snapshot 导出。
  - 单进程崩溃 = 数据全丢。
- ❌ **无水平扩展**:
  - 没有 `--cluster`、没有 gossip、没有 consistent hashing、没有 leader election。
  - `/api/services` 与 `/api/stream` 是单进程内的 hub(`stream.Hub`),cluster 视角不存在。
- ❌ **无复制 / 主备 / Raft / etcd 集成**。
- ❌ **无 multi-region / geo-routing**。`grep "multi.region|HA|disaster.recovery|failover|replication" *.md` 0 命中。
- ⚠️ 部署模式只有:local run / docker compose 两节点 / 通用 ingress。**没有跨 AZ、跨 region 的部署建议**。
- ❌ **无备份 / restore SOP**。项目本身体现为"易失缓存",用作演示这没问题,但生产级观测平台必须支持 archive / restore。

### 9. 可观测性自身(self-observability)

- ⚠️ SDK 自带 `Stats()` 12 个原子计数器,好用;后端 `/api/health` 与 `/api/services` 是状态面。
- ❌ **Demo-dog 自身没有暴露 Prometheus `/metrics` 端点**。README 路线图已标记 TODO:"在后端增加 Prometheus /metrics 端点"。
- ❌ 后端启动横幅只有 5 行指标(workers / queue depth / TTL / 容量),**无 metrics 可被 scrape**。
- ❌ 没有自描述的服务发现 manifest(`/etc/otel/...`、`/.well-known`)。

---

## C. 整体定性结论

| 评级维度 | 评估 |
|---|---|
| **定位** | 明确为 **教学/演示** 项目(README 自承 MIT"仅用于教学与演示目的",roadmap 多处自标 TODO),**不是企业级生产级**。 |
| **完成度** | 单进程内存版 Doris + 单语言 Go SDK + 单体前端,做到了 **一个具体 demo 的完整闭环**。 |
| **可演进到企业级** | 路线图涵盖 OTLP Protobuf、Helm Chart、Prometheus `/metrics`、React Router 等,说明作者清楚缺口。但当前缺的关键能力太多。 |

### 阻塞企业使用的 Top-10 Gap 列表

| # | 缺口 | 影响 |
|---|---|---|
| 1 | **零持久化 / 零 HA** | 重启 = 数据全丢,无法 SLA |
| 2 | **无 auth 也不支持 mTLS** | 不能进内网采集 |
| 3 | **只有 Go SDK** | 多语言栈无法接入 |
| 4 | **不支持 OTLP/gRPC** | 上游 OTel SDK 默认 gRPC,需要 proxy 翻译 |
| 5 | **Histogram 无 buckets** | P95/P99 算不出来 |
| 6 | **无 tail-based / 远程 / 错误采样** | 大量 trace 浪费带宽 |
| 7 | **无 K8s manifests / Helm Chart** | 不能 kubectl apply |
| 8 | **无 CI/CD 工作流** | 升级混乱 |
| 9 | **LICENSE 文件缺失** | 法律风险 |
| 10 | **无 CHANGELOG / ADR / CONTRIBUTING** | 维护性差 |

---

## D. 简单"还在工作"的部分(快速过)

| 维度 | 状态 |
|---|---|
| SDK 简化 envelope 序列化 | ✅ |
| 后端 OTLP JSON 解码 + 异步工作池 + 内存 Doris | ✅ |
| `go test -race` 后端测试 | ✅ |
| 多阶段镜像(Alpine ~25 MB image) | ✅ |
| 健康探针 + Docker healthcheck | ✅ |
| 800+ 行 README mermaid,可视化极强 | ✅ |
| W3C Trace Context 注入/提取 | ✅ |
| 一套 InMemoryCollector 单元测试助手 | ✅ |
| Zero SDK 外部依赖(纯标准库) | ✅ |
| 字节/hertz/字节生态示例有覆盖 | ✅ |

---

> 备注:所有结论基于本仓库当前快照。`examples/` 总计 17 个子目录,涵盖 gin/hertz/beego/echo/middleware/db-tracing/kafka-consumer/grpc-server/frontend-log-pipe/trace-propagation/otel-bridge/prometheus-exporter/sampler-debug/loadtest/worker-loop/quickstart/demo-dog —— 数量充足,但都是教学示例,**与"企业 SDK 应交付的能力矩阵"之间仍有显著距离**。
