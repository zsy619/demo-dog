# Internal 包归类总览

`internal/` 下的所有包按功能主题划分到 7 个 **x*** 子树中：

| 主题    | 子包数 | 职责 |
|---------|--------|------|
| xcache  | 35     | 缓存淘汰算法、限流、断路器、布隆过滤器、KV 缓存 |
| xcore   | 26     | 运行时、可观测性（log/metrics/tracing/slo/sampler/audit/health） |
| xdata   | 56     | 持久化数据结构：WAL、KV、索引、B+ 树、跳表、租约、模型、存储引擎 |
| xflow   | 42     | 并发编排：池、队列、信号量、栅栏、合并、调度、重试、saga、路由 |
| xnet    | 29     | HTTP / TLS / 代理 / 路由 / 健康探测 / OpenAPI / Webhook |
| xsecure | 27     | 认证、加密、令牌、密码、策略、签名、Nonce、能力、RBAC |
| xtool   | 44     | 通用工具：类型转换、字符串、集合、位集、堆、定时轮、SQL 构造器 |

总计 **259** 个子包，**257** 个测试包全部 `go test -race` 通过。

---

## xcache — 缓存 / 限流 / 淘汰算法

### 缓存策略（替换算法变体）
- `arc` — 自适应替换（ARC）
- `clock` / `clocksweep` — CLOCK 二次机会算法
- `fifo` — 先进先出
- `lfu` — 最不经常使用
- `tinylfu` — TinyLFU 准入策略
- `lru2c` — 2Q LRU 近似
- `probation` — W-TinyLFU 二级保护
- `window` / `windowx` — 滑动窗口
- `recency` / `freq` — LRU/LFU 频次视图辅助
- `warm` — 缓存预热协调器

### KV 缓存容器
- `cache` — 通用缓存（GetOrLoad）
- `ttlcache` — 带 TTL 的内存缓存
- `tinycache` — 极简 string->string 缓存
- `keycache` — 高并发 map + 简单 TTL
- `rwcache` — RWMutex 并发缓存（含统计）
- `seqcache` — 按 sequence 顺序淘汰
- `sizelim` — 按字节大小限制
- `ringcache` — RingBuffer 覆盖式
- `segment` — 按前缀分段 KV
- `qcache` — 按 query key 缓存函数调用
- `proxy_cache` — 本地 + 远程二层缓存
- `namedcache` — 按名字隔离的多个 LRU

### 限流 / 断路
- `ratelimit` — 令牌桶 / 漏桶
- `sliding` — 滑动窗口限流
- `quota` — 多 key 配额管理器
- `iplimit` — IP 级限流
- `sempool` — 权重资源信号量
- `bulkhead` — 隔板限流
- `breaker` — 断路器（含 Stats）
- `circuit` — 断路器（轻量 API，含 Snapshot）

### 其它
- `bloom` — 布隆过滤器
- `exactly` — exactly-once 标记

---

## xcore — 运行时 / 可观测性

### 可观测性
- `logx` — 结构化日志接口
- `metrics` — Prometheus 指标注册器
- `tracing` — 调用链路追踪
- `sampler` — 概率采样器
- `slo` — SLO 追踪
- `audit` — 审计事件日志
- `health` — 健康检查端点

### 运行时
- `config` — 配置加载（环境变量/文件/默认值）
- `cron` — 定时任务
- `lifecycle` — 进程生命周期钩子
- `grace` — 优雅停机协调器
- `inspectx` — 进程运行时信息快照
- `procstate` — 结构化进程状态输出
- `runtimex` — 运行时错误与告警事件收集
- `profmem` / `pprofx` — 内存/pprof 分析辅助

### 辅助工具
- `clockx` — 可注入时钟抽象（便于 time 注入测试）
- `bufferx` — 线程安全字节 buffer
- `loggerx` — 轻量结构化日志接口（与 logx 互补）
- `metricsx` — 浮点原子转换
- `notify` — 主题无关事件总线
- `pipelinex` — 上下文驱动的 Pipeline
- `reflection` — 类型名/字段遍历
- `spin` — 自旋等待与自适应退避
- `timermx` — 定时任务管理器

### 调试
- `debug` — 调试端点（pprof 之外）

---

## xdata — 数据结构 / 持久化

### 模型与状态
- `model` — 三大可观测性支柱的共享数据模型
- `feature` — 特性开关（应用层）
- `feature_flag` — 内存版特性开关存储
- `tenants` — 多租户注册表
- `registry` — 通用服务注册表
- `topo` — 拓扑/层级结构

### 存储抽象
- `store` — Apache Doris 风格内存存储引擎（5130 行，最大子包）
- `ingest` — OTLP 风格 JSON 写入流水线
- `shardkv` — 分片 KV（key 哈希到独立 map）
- `dualstore` — 主备模式 KV（双写 + 读优先主）
- `sortedkv` — 按 key 排序的 KV
- `partition` — 一致性哈希分片
- `versionstore` — 多版本数据存储（保留最近 N 版本）
- `versioning` — 单调递增版本号乐观锁
- `persist` — 基于 binary 的 KV 文件持久化
- `kvsafe` — 加密 KV 持久化保险箱
- `vault` — 凭证保管

### 索引
- `index` — 内存 B-Tree 索引
- `bptree` — 内存 B+ 树
- `skiptable` — 跳表
- `idxmap` — 倒排索引
- `idx` — 通用索引抽象
- `scanmap` — 带游标的 map 扫描（分页遍历）

### 树 / 图
- `merkle` — 默克尔树
- `lsm` — LSM 结构

### 复制 / 日志
- `wal` — 预写日志
- `dualwal` — 双写 WAL
- `replica` — 主备复制（1689 行）
- `journal` — KV 变更日志
- `cdc` — 变更数据捕获
- `migrate` — KV 数据迁移协调器
- `retention` — 数据保留策略
- `softdel` — 软删除

### 一致性 / 时间
- `vclock` — 向量时钟
- `lease` — 租约
- `idempotency` — 幂等键管理

### 集合
- `bitmap` — 简单位图
- `idset` — int64 ID 集合（位图）
- `bloomx` — 可序列化布隆过滤器
- `listx` — 双向链表
- `rwlist` — 读写锁双向链表
- `stackx` — 泛型栈
- `queuex` — 并发有界环形队列
- `ringqueue` — 固定容量环形队列
- `rwqueue` — 多生产者多消费者队列
- `circular` — 循环缓冲
- `bigcounter` — big.Int 并发计数器
- `syncmap` — 轻量并发泛型 map

### 区间与段
- `segmentx` — 区间段（半开区间 [lo, hi)）管理

### 序列化
- `serialize` — binary 编解码
- `snapshot` — JSON 快照（含 checksum + gzip）

### 缓存辅助
- `lrumap` — 泛型并发 LRU
- `lruview` — 最小 LRU
- `tscache` — 键-时间序列缓存（监控用）

### 差异
- `diff` — JSON 表示差分计算

---

## xflow — 并发编排 / 控制流

### 池 / 队列 / 工作者
- `pool` — 通用 goroutine 池（367 行）
- `wpool` — worker 池（300 行）
- `opool` — 对象池（142 行）
- `batch` — 有界 worker 池（OTLP ingest 用）
- `poolqueue` — 对象池化的队列（复用元素）
- `poolg` — 固定大小 goroutine 工作池
- `poolx` — sync.Pool 对象池辅助（自动 Reset）
- `executor` — 固定 worker 数量任务执行器
- `worksteal` — 工作窃取调度
- `workqueue` — 带延迟的优先级工作队列
- `prioq` — 优先队列 + 批处理
- `queue` — 线程安全 FIFO 队列
- `scheduler` — 简单任务调度器

### 同步原语
- `semaphore` — 加权计数信号量
- `signalx` — 高粒度信号量
- `barrier` — 并发栅栏（N 个协程全部到达）
- `latch` — CountDownLatch / StartLatch
- `spinlock` — atomic 自旋锁
- `timeout` — 超时装饰器

### 并发编排
- `group` — errgroup 风格并发编排
- `parallel` — 并行执行辅助（panic 恢复 + ctx）
- `mergex` — 合并器
- `singleflight` — 泛型 SingleFlight

### 异步
- `promise` — 一次性异步结果封装
- `pubsub` — 进程内 pubsub
- `stream` — 内存 hub（流式推/拉）
- `broadcast` — 多接收者广播（订阅 + 发布）
- `watchx` — 简单发布/订阅监视器
- `triggerx` — 事件触发器
- `cb` — 通用回调注册中心

### 重试 / 弹性
- `retry` — 指数退避 + 抖动可重试执行
- `debounce` — 函数防抖/节流
- `recurring` — 周期任务触发器
- `backp` — 背压感知字节通道
- `timeoutlearner` — 自适应超时学习

### 业务流程
- `saga` — 分布式事务 saga
- `pipeline` — Pipeline 模式
- `term` — 终止信号
- `alerts` — 告警规则引擎（应用层，1679 行）

### 路由 / 匹配
- `router` — HTTP 路由
- `radix` — Radix 树匹配
- `routerx` — Trie 树 HTTP 路由

### 帧 / 文件
- `framing` — 帧协议
- `filewatch` — 轮询文件监听

### 工具
- `seq` — 单调递增原子序号生成器

---

## xnet — 网络 / HTTP / TLS

### HTTP 服务端
- `api` — HTTP API 层（8312 行，最大子包）
- `middleware` — HTTP 中间件链接器
- `reqlog` — HTTP 请求日志
- `gzipm` — gzip 压缩中间件
- `headers` — 常用 HTTP Header 常量
- `cookiex` — Cookie 解析与序列化
- `mimex` — MIME 类型推断

### HTTP 客户端
- `httpxx` — net/http 客户端封装
- `dialer` — 按 host 跟踪指数退避拨号冷却
- `dialerx` — Dialer 构建辅助
- `proxy` — HTTP/SOCKS 代理
- `proxyx` — 代理 URL 解析与构建

### 路由
- `routex` — 最长前缀匹配路由表
- `loadbal` — 客户端负载均衡器

### TLS / 证书
- `tlsconfig` — TLS 配置
- `certmini` — PEM 解析与自签名生成
- `certpin` — (host, fingerprint) 证书钉扎
- `jwt` — JWT 签发校验（应用层）
- `jwt2` — HTTP Bearer Token 提取 + JWKS
- `oauth` — OAuth 授权码流程

### 健康 / 探测
- `probe` — 主动健康探测
- `tcphealth` — TCP 健康探测
- `keepalive` — keepalive ping

### 协议基础
- `connpool` — 连接池
- `cidr` — CIDR/IP 网段解析
- `resolverx` — DNS 解析 + 缓存
- `ring` — 一致性哈希环

### API 文档
- `openapi` — OpenAPI 3.1 spec 生成

### 回调
- `webhook` — Webhook 回调发送

---

## xsecure — 认证 / 加密 / 鉴权

### 主入口
- `auth` — 鉴权主入口（1741 行，含 OIDC）
- `rbac` — RBAC 引擎
- `rbacx` — 轻量 RBAC 引擎（精简版）
- `session` — 会话管理
- `abac` — 属性访问控制

### 加密
- `cipher` — AES-GCM 包装
- `cipherio` — AES-CTR 流式加解密
- `xorx` — 简化 XOR 流密码
- `ed25519x` — Ed25519 签名/校验
- `macx` — HMAC-SHA256/512/SHA1
- `digest` — HTTP Digest 鉴权
- `xhash` — md5/sha1/sha256/sha512/fnv 汇总
- `base58x` — Base58 编解码
- `password` — 密码哈希（PBKDF2-SHA256，OWASP 600k iter）
- `randomx` — crypto/rand 安全随机数

### 令牌 / 凭证
- `token` — 一次性令牌（OTP）
- `tokenx` — HMAC 签名 token
- `api_key` — API Key 签发与校验
- `oauth2` — OAuth2 授权码流程
- `cap` — 能力令牌（Capability Token）
- `nonce` — Nonce 防重放
- `totp` — 时间一次性密码（RFC 6238）

### 密钥管理
- `vault` — 凭证保管
- `secretrot` — 密钥轮换

### 审计与策略
- `auditx` — 内存审计事件日志
- `policy` — 策略表达式求值器
- `sig` — 签名校验

### 辅助
- `entropy` — 香农熵（密码强度评估）

---

## xtool — 通用工具

### 类型 / 转换
- `boxx` — 类型安全容器 / Box
- `optional` — Option[T] 泛型（区分"未设置"与零值）
- `conv` — 常见类型安全转换
- `convx` — 字符串解析与类型转换
- `copyx` — gob 深拷贝
- `cmp` — 通用比较器

### 字符串
- `stringsx` — 高效字符串拼接
- `strutil` — 常用字符串处理
- `mustx` — panic on error 风格辅助

### 集合
- `sets` — 集合操作（基于 map）
- `bitset` — 位图集合
- `bitsetx` — 固定大小位集
- `orderedmap` — 保留插入顺序的并发 map
- `rangeutil` — 区间合并与覆盖判定

### 序列化 / 编解码
- `dump` — JSON / 表格 / 缩进打印
- `printx` — 格式化输出（表格、字段、列表）
- `hexdump` — xxd/hexdump 格式输出
- `encodex` — base64/base32/base58/hex 互转
- `heapx` — container/heap 包装的优先队列

### 错误
- `errorsx` — 错误分类、聚合、上下文包装
- `errs` — 结构化错误码 + 消息

### 调度
- `timernx` — 时间轮（Timing Wheel）
- `cronx` — 5 段 cron 表达式最小匹配

### 缓存辅助
- `memox` — 键值备忘存储（含 LastAccess）
- `inspect` — 任意值反射检视

### 指标 / 调试
- `metricx` — 轻量级指标推送器

### 配置 / 标志
- `flagx` — 环境变量 / 文件 / 命令行获取

### HTTP / 文件
- `httpx` — 带超时与重试的 HTTP 客户端
- `docstore` — 文档存储辅助
- `tarx` — tar 扩展

### 身份
- `uuid` — UUID v4

### URL / 路径
- `urlx` — net/url 扩展

### 配置格式
- `yamlx` — 最小化 YAML 序列化器

### 测试
- `e2e` — 端到端测试辅助
- `smoke` — 烟雾测试

### 数据
- `sqlb` — 白名单 SQL 字符串构造器
- `geo` — 地理编码

### 协议
- `tpc` — 三方提交协议

---

## 设计原则

1. **主题先行**：每个 x* 目录承担一个明确的语义主题，包名应与目录名一致
2. **std-lib only**：不引入第三方依赖
3. **原子化**：每个子包专注一个明确职责，避免 god package
4. **算法变体共存**：CLOCK/CLOCKSweep/FIFO/LFU/LRU 等同类算法的不同实现都保留，作为对比参考
5. **避免重复抽象**：高频使用的工具（如 breaker/circuit）保留主实现 + 备选 API；不轻易删除
