#!/usr/bin/env python3
"""批量翻译 Go 注释: 纯英文 -> 简体中文"""
import re
import sys
import os

# 词汇级映射
TERMS = {
    "WAL": "WAL",
    "LSM": "LSM",
    "B+ tree": "B+ 树",
    "Merkle tree": "默克尔树",
    "goroutine": "协程",
    "channel": "通道",
    "mutex": "互斥锁",
    "semaphore": "信号量",
    "backpressure": "背压",
    "idempotency": "幂等",
    "lease": "租约",
    "vector clock": "向量时钟",
    "snapshot": "快照",
    "audit": "审计",
    "middleware": "中间件",
    "proxy": "代理",
    "rate limit": "限流",
    "rate-limit": "限流",
    "rate-limited": "限流的",
    "rate-limiter": "限流器",
    "token bucket": "令牌桶",
    "leaky bucket": "漏桶",
    "circuit breaker": "断路器",
    "worker pool": "工作池",
    "queue": "队列",
    "ring buffer": "环形缓冲",
    "backoff": "退避",
    "retry": "重试",
    "histogram": "直方图",
    "summary": "汇总",
    "percentile": "百分位",
    "tenant": "租户",
    "OTLP": "OTLP",
    "PromQL": "PromQL",
    "SLO": "SLO",
    "HTTP": "HTTP",
    "HTTPS": "HTTPS",
    "TLS": "TLS",
    "TCP": "TCP",
    "UDP": "UDP",
    "DNS": "DNS",
    "JWT": "JWT",
    "OAuth": "OAuth",
    "OIDC": "OIDC",
    "mTLS": "mTLS",
    "RBAC": "RBAC",
    "ABAC": "ABAC",
    "HMAC": "HMAC",
    "PBKDF2": "PBKDF2",
    "SPIFFE": "SPIFFE",
    "SAML": "SAML",
    "CSRF": "CSRF",
    "XSS": "XSS",
    "API": "API",
    "SDK": "SDK",
    "CLI": "CLI",
    "GUI": "GUI",
    "URL": "URL",
    "URI": "URI",
    "JSON": "JSON",
    "YAML": "YAML",
    "SQL": "SQL",
    "NoSQL": "NoSQL",
    "OLAP": "OLAP",
    "OLTP": "OLTP",
    "CRC": "CRC",
    "MVCC": "MVCC",
    "MV": "物化视图",
    "materialized view": "物化视图",
    "cardinality": "基数",
    "label set": "标签集",
    "span": "span",
    "trace": "trace",
    "exemplar": "exemplar",
    "scrape": "抓取",
    "drain": "排空",
    "graceful shutdown": "优雅停机",
    "replica": "副本",
    "primary": "主",
    "follower": "从",
    "leader": "leader",
    "election": "选举",
    "compaction": "压缩",
    "retention": "保留",
    "sidecar": "sidecar",
    "mesh": "mesh",
    "gateway": "网关",
    "sidecar": "sidecar",
    "hot tier": "热层",
    "cold tier": "冷层",
    "warm tier": "温层",
    "WebSocket": "WebSocket",
    "SQLite": "SQLite",
    "Postgres": "Postgres",
    "MySQL": "MySQL",
    "Kafka": "Kafka",
    "Redis": "Redis",
    "Prometheus": "Prometheus",
    "OpenTelemetry": "OpenTelemetry",
    "Doris": "Doris",
}

# 短语级映射(更具体)
PHRASES = {
    # store/doris.go
    "The design intentionally mirrors real Doris concepts so that the demo behaves like an OLAP backend, even though everything fits in RAM:": "本实现刻意映射真实 Doris 概念,使 demo 表现为 OLAP 后端(尽管全部在内存中):",
    "Each signal lives in its own table: __dog_logs, __dog_metrics, __dog_traces.": "每个 signal 拥有独立表:__dog_logs、__dog_metrics、__dog_traces。",
    "Hot/cold tiering: the most recent N records are kept in a \"hot\" partition and the older ones spill into a \"cold\" partition. Queries report which tier they hit so the frontend can hint about latency.": "热/冷分层:最近 N 条记录保留在 \"hot\" 分区,更老的进入 \"cold\" 分区。查询会上报命中层以便前端提示延迟。",
    "Materialized views: simple bucket-level aggregations (logs per service, metric 1m rollup) are pre-computed and labeled by window.": "物化视图:简单的桶级聚合(按服务的日志、按分钟的指标汇总)预先计算并按窗口标记。",
    "Pseudo-indexes: hash buckets + range indices on timestamp + service.": "伪索引:时间戳 + 服务维度的哈希桶 + 范围索引。",
    "Concurrency: every public method is safe for concurrent use. We rely on single-writer-multiple-readers semantics by guarding mutations with a mutex and bucket-level locking for the hot tier.": "并发:每个公开方法并发安全。采用单写多读语义——写操作由互斥锁保护,热层另加桶级锁。",
    "DefaultConfig returns sensible defaults for the demo.": "DefaultConfig 返回适合 demo 的合理默认。",
    "Validate returns the first config problem or nil.": "Validate 返回首个配置问题或 nil。",
    "Callers should fail fast at startup.": "调用方应在启动时快速失败。",
    "Doris is the in-memory engine for the demo.": "Doris 是 demo 的内存引擎。",
    "New returns a freshly initialized engine.": "New 返回一个新初始化的引擎。",
    "Stats counters surfaced through /api/health.": "通过 /api/health 暴露的统计计数器。",
    "Stats returns a snapshot of the engine counters.": "Stats 返回引擎计数器的快照。",
    "InsertLogs performs a Doris-style Stream Load of log records.": "InsertLogs 执行 Doris 风格的 Stream Load 日志记录。",
    "It returns the number of accepted rows.": "返回接受的行数。",
    "InsertMetrics adds metric points and updates minute-level MVs.": "InsertMetrics 添加指标点并更新分钟级物化视图。",
    "When the engine has reached cfg.MaxCardinality, new (label-set) writes return ErrCardinalityFull without being indexed.": "当引擎达到 cfg.MaxCardinality 时,新的标签集写入会返回 ErrCardinalityFull 且不入索引。",
    "InsertSpans adds trace spans.": "InsertSpans 添加 trace span。",
    "QueryLogs / QueryMetrics / QuerySpans: filter + paginate over the corresponding tier.": "QueryLogs / QueryMetrics / QuerySpans:对相应层做过滤 + 分页。",
    # signal.go
    "Histogram View returned by aggregate roll-ups.": "聚合汇总返回的直方图视图。",
    "Histogram bucket bounds for SLO p95/p99.": "SLO p95/p99 用的直方图分桶边界。",
    "Sum of all values seen in the window.": "窗口内所有值之和。",
    "Sample count.": "采样数。",
    "Recent values retained for percentile calculation.": "为百分位计算保留的最近值。",
    # otlp.go
    "Tenant id (or empty = all)": "租户 ID(或留空表示全部)",
    "exact match (or empty)": "精确匹配(或留空)",
    "metric name (or empty)": "指标名(或留空)",
    "log severity (or empty) — exact match": "日志严重等级(或留空),精确匹配",
    "when true, Severity is a >= comparison against severity ordering": "为 true 时,Severity 按等级做 >= 比较",
    "0=TRACE … 5=FATAL; used when SeverityAtLeast is true": "0=TRACE … 5=FATAL;当 SeverityAtLeast 为 true 时使用",
    "trace id (or empty)": "trace ID(或留空)",
    "substring search across log bodies / span names": "对日志正文 / span 名做子串搜索",
    "every key must be present with the same value": "每个 key 必须存在且取值相同",
    "include records with ts >= SinceMs (0 = no lower bound)": "包含 ts >= SinceMs 的记录(0 表示无下限)",
    "include records with ts <= UntilMs (0 = no upper bound)": "包含 ts <= UntilMs 的记录(0 表示无上限)",
    "hard cap on rows returned": "返回行数的硬上限",
    "\"1m\" / \"5m\" for metrics MV selection": "\"1m\" / \"5m\" 用于指标物化视图选择",
    # store/extra.go
    "top services by request count in the given window": "在指定窗口内按请求数排序的 top 服务",
    "These are pseudo-services generated from the cardinality gate.": "这些是由基数门控产生的伪服务。",
    "TopOps surfaces the top N operations per service.": "TopOps 输出每个服务的 top N 操作。",
    # ingest/otlp.go
    "otlp format strings reused across ingestion paths": "OTLP 格式字符串,跨接入路径复用",
    "rec format": "rec 格式",
    "snapshot is a compressed in-memory state dump for WAL.": "快照是压缩后的内存状态转储,用于 WAL。",
    # xflow/term.go
    "the term number": "任期号",
    "voted for in this term": "本任期投给了谁",
    "highest log index": "最高日志索引",
    "highest committed index": "最高已提交索引",
    "highest applied index": "最高已应用索引",
    # xflow/alerts/notify.go
    "subject of the alert": "告警主题",
    "summary field of the alert": "告警摘要字段",
    "severity label for the alert": "告警严重等级标签",
    "single message notification": "单条消息通知",
    "deployment name for routing": "用于路由的部署名",
    "secret reference name": "secret 引用名",
    # Common
    "controlled by config": "由配置控制",
    "default": "默认值",
    "may be nil": "可为 nil",
    "returns the": "返回",
    "the previous": "前一个",
    "the next": "下一个",
    "the latest": "最近的",
    "the oldest": "最早的",
    "a new": "新的",
    "an empty": "空的",
    "when set": "设置时",
    "when nil": "nil 时",
    "if non-empty": "若非空",
    "if set": "若已设置",
    "if nil": "若 nil",
    "deprecated": "已弃用",
}

# 通用短语替换
GENERIC = [
    ("used for", "用于"),
    ("used to", "用于"),
    ("used by", "由...使用"),
    ("in seconds", "(秒)"),
    ("in milliseconds", "(毫秒)"),
    ("in bytes", "(字节)"),
    ("at most", "最多"),
    ("at least", "至少"),
    ("service name", "服务名"),
    ("metric name", "指标名"),
    ("span name", "span 名"),
    ("trace id", "trace ID"),
    ("log body", "日志正文"),
    ("start time", "开始时间"),
    ("end time", "结束时间"),
    ("created at", "创建时间"),
    ("updated at", "更新时间"),
    ("deleted at", "删除时间"),
    ("expires at", "过期时间"),
    ("the number of", "数量"),
    ("number of", "数量"),
    ("returns", "返回"),
    ("creates", "创建"),
    ("sends", "发送"),
    ("receives", "接收"),
    ("writes", "写入"),
    ("reads", "读取"),
    ("starts", "启动"),
    ("stops", "停止"),
    ("registers", "注册"),
    ("removes", "移除"),
    ("checks", "检查"),
    ("records", "记录"),
    ("emits", "发出"),
    ("handles", "处理"),
    ("processes", "处理"),
    ("err", "错误"),
    ("req", "请求"),
    ("resp", "响应"),
    ("fn", "函数"),
    ("ctx", "上下文"),
    ("cfg", "配置"),
    ("param", "参数"),
    ("msg", "消息"),
    ("num", "数值"),
    ("cnt", "计数"),
    ("buf", "缓冲"),
    ("mux", "多路复用器"),
    ("mgr", "管理器"),
    ("svc", "服务"),
]

def has_cjk(s):
    return any('\u4e00' <= c <= '\u9fff' for c in s)

def translate_comment(text):
    """翻译一条注释行(不含 // 前缀)"""
    # 先查 phrases
    s = text
    # 应用短词 - 单词边界
    for en, zh in sorted(PHRASES.items(), key=lambda x: -len(x[0])):
        s = s.replace(en, zh)
    # 应用通用短语
    for en, zh in GENERIC:
        # 单词边界匹配
        s = re.sub(r'\b' + re.escape(en) + r'\b', zh, s)
    # 应用术语 - 仅作为完整单词
    for en, zh in TERMS.items():
        if en != zh:
            # 跳过纯英文专有名词
            s = re.sub(r'\b' + re.escape(en) + r'\b', zh, s)
    return s

def process_file(path):
    """处理一个文件,返回 (changed, orig_lines, new_lines)"""
    with open(path, encoding='utf-8') as f:
        lines = f.readlines()
    new_lines = []
    changed = False
    for line in lines:
        stripped = line.lstrip()
        if stripped.startswith('//'):
            # 提取注释
            indent_len = len(line) - len(stripped)
            indent = line[:indent_len]
            comment_body = stripped[2:].lstrip()  # 去掉 //
            # 跳过空注释
            if not comment_body.strip():
                new_lines.append(line)
                continue
            # 跳过版权/SPDX
            lower = comment_body.lower()
            if any(kw in lower for kw in ['copyright', 'spdx', 'apache license', 'license']):
                new_lines.append(line)
                continue
            # 已有中文 - 跳过
            if has_cjk(comment_body):
                new_lines.append(line)
                continue
            # 翻译
            translated = translate_comment(comment_body)
            # 保留尾部换行
            trailing = line[len(line)-len(line.rstrip('\n')):] if line.rstrip() else '\n'
            if line.endswith('\n'):
                trailing = '\n'
            else:
                trailing = ''
            if not trailing and not line.endswith('\n'):
                trailing = ''
            # 重新构造
            prefix = indent + '// '
            new_line = prefix + translated.rstrip() + '\n' if line.endswith('\n') else prefix + translated.rstrip()
            if translated != comment_body:
                changed = True
            new_lines.append(new_line)
        else:
            new_lines.append(line)
    return changed, new_lines

if __name__ == '__main__':
    total_changed = 0
    total_files = 0
    for path in sys.argv[1:]:
        if path.endswith('_test.go'):
            continue
        if not os.path.exists(path):
            continue
        changed, new_lines = process_file(path)
        if changed:
            with open(path, 'w', encoding='utf-8') as f:
                f.writelines(new_lines)
            total_files += 1
            total_changed += sum(1 for o, n in zip(open(path, encoding='utf-8').readlines(), new_lines) if o != n)
    print(f'修改了 {total_files} 个文件')
