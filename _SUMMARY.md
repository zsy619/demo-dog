# demo-dog 项目评测报告(已写入 SDK_EVALUATION.md)

完整报告已写入 `/Volumes/E/JYW/创意项目/demo-dog/SDK_EVALUATION.md`(18.5 KB, 256 行)。

## 关键发现摘要

### 项目定位
**自述为"教学/演示"项目**(README 明文:"MIT —— 仅用于教学与演示目的"),**不是企业级生产级**。但仓库仍存在大量尚未覆盖的企业级能力缺口。

### A. SDK 能力差距(10 个维度)

| # | 维度 | 关键缺口 |
|---|---|---|
| 1 | 语言覆盖 | ❌ **只有 Go SDK**,无 Java/Node/Python/.NET/PHP/Ruby |
| 2 | 协议支持 | ✅ 简化 JSON + 标准 OTel JSON + Prometheus text,**❌ 无 OTLP/gRPC、Zipkin、Jaeger Thrift、Prometheus Remote Write、OTLP/Protobuf** |
| 3 | 自动埋点 | ❌ **无 agent 模式**,无 OTel API 兼容层,SDK 是零依赖纯手写 API |
| 4 | 资源检测 | ❌ **无 K8s pod labels、无 cloud metadata、无 container ID**,只有 process.* + host.arch |
| 5 | 采样智能 | ✅ 5 个 Sampler(Always/Ratio/Parent/Composite),**❌ 无 tail-based、错误采样、远程采样** |
| 6 | 批量与背压 | ⚠️ buffer **无容量上限**,**无指数退避、无 circuit breaker、无死信、无持久化** |
| 7 | 指标丰富度 | ⚠️ Histogram 是单点 sample,**❌ 无 buckets、无 exemplars、无 exponential histogram、无 Summary 类型** |
| 8 | 日志功能 | ✅ 6 级 severity,**❌ 无 appender、无运行期开关、Body 强类型 string、无 trace 自动注入** |
| 9 | 配置 | ✅ 函数式 Option,**❌ 完全无环境变量读取(YAML/TOML/Remote/Discovery)** |
| 10 | 安全 | ❌ **无 mTLS、无 auth 注入、无 PII 脱敏,后端 /api/ingest/otlp 完全开放** |

### B. 工程/部署/治理差距

| 维度 | 状态 |
|---|---|
| Docker | ✅ 多阶段 Dockerfile + 2 个 docker-compose(nginx/Caddy) |
| K8s/Helm | ❌ **完全没有任何 k8s manifests 或 Helm chart**(README 路线图 TODO) |
| CI/CD | ❌ **无任何 CI 配置文件**(无 .github/.gitlab-ci/Jenkinsfile) |
| ADR | ❌ 无 ARCHITECTURE.md / docs/adr |
| License | ⚠️ README 声明 MIT **但 LICENSE 文件缺失**(法律风险) |
| Contributing | ❌ 无 CONTRIBUTING/CODE_OF_CONDUCT/SECURITY.md |
| Changelog | ❌ 无 CHANGELOG.md,无 release tag |
| SemVer/稳定 API | ❌ 无 SemVer 约定,无 stable API promise |
| HA/灾备 | ❌ **后端纯 in-memory,无持久化、无水平扩展、无主备、无备份** |
| Self-metrics | ❌ 后端无 Prometheus /metrics 端点(README TODO) |

### Top-10 阻塞企业使用的缺口

1. 零持久化/零 HA → 重启数据全丢
2. 无 auth/mTLS → 不能进内网
3. 只 Go SDK → 多语言栈无法接入
4. 无 OTLP/gRPC → 上游 OTel SDK 默认 gRPC
5. Histogram 无 buckets → P95/P99 算不出
6. 无 tail-based/远程采样 → 浪费带宽
7. 无 K8s/Helm → 不能 kubectl apply
8. 无 CI/CD → 升级混乱
9. LICENSE 文件缺失 → 法律风险
10. 无 CHANGELOG/ADR/CONTRIBUTING → 维护性差

### 已成功实现的部分(快速过)

✅ 简化/标准 OTel envelope 序列化 · ✅ W3C Trace Context · ✅ 后端异步工作池 · ✅ 内存版 Doris(冷热分层+物化视图) · ✅ 多阶段 Alpine 镜像 ~25 MB · ✅ 健康探针 · ✅ 800+ 行 README mermaid 可视化 · ✅ 17 个示例(gin/hertz/beego/echo/db/kafka 等) · ✅ Zero SDK 外部依赖 · ✅ go test -race 后端单元测试