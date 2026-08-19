package io.demodog.sdk;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

/**
 * demo-dog Java SDK.
 *
 * Stdlib-only: uses java.net.http for the HTTP client and a single
 * scheduled thread for buffer flushing. Suitable for inclusion in
 * any service running on JDK 11+.
 *
 * <pre>{@code
 *   try (DemoDogClient c = DemoDogClient.builder()
 *       .baseUrl("http://localhost:18080")
 *       .apiKey(System.getenv("DOG_API_KEY"))
 *       .service("checkout")
 *       .tenant("acme")
 *       .build()) {
 *       c.log("INFO", "hello");
 *       c.counter("orders.placed", 1.0);
 *       c.histogram("checkout.duration_ms", 78.5);
 *       c.span("trace-1", "span-1", "GET /checkout", 120, "ok");
 *   }
 * }</pre>
 */
public final class Client implements AutoCloseable {
    private final String baseUrl;
    private final String apiKey;
    private final String service;
    private final String version;
    private final String tenant;
    private final int maxBuffer;
    private final ConcurrentLinkedQueue<LogRecord> logs = new ConcurrentLinkedQueue<>();
    private final ConcurrentLinkedQueue<MetricPoint> metrics = new ConcurrentLinkedQueue<>();
    private final ConcurrentLinkedQueue<SpanRecord> spans = new ConcurrentLinkedQueue<>();
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).build();
    private final ScheduledExecutorService flusher = Executors.newSingleThreadScheduledExecutor(r -> {
        Thread t = new Thread(r, "demo-dog-flusher");
        t.setDaemon(true);
        return t;
    });
    private volatile boolean closed = false;

    private Client(String baseUrl, String apiKey, String service, String version, String tenant, int maxBuffer, long flushIntervalMs) {
        this.baseUrl = baseUrl;
        this.apiKey = apiKey;
        this.service = service;
        this.version = version;
        this.tenant = tenant;
        this.maxBuffer = maxBuffer;
        if (flushIntervalMs > 0) {
            flusher.scheduleAtFixedRate(this::flushSafe, flushIntervalMs, flushIntervalMs, TimeUnit.MILLISECONDS);
        }
    }

    public static Builder builder() { return new Builder(); }

    public void log(String severity, String body) { log(severity, body, null); }

    public void log(String severity, String body, Map<String, String> attributes) {
        logs.add(new LogRecord(System.currentTimeMillis() * 1_000_000L, severity, body, attributes == null ? Map.of() : attributes));
        maybeDrop();
    }

    public void counter(String name, double value) { counter(name, value, null); }
    public void counter(String name, double value, Map<String, String> attributes) {
        emitMetric(name, value, attributes);
    }

    public void gauge(String name, double value) { gauge(name, value, null); }
    public void gauge(String name, double value, Map<String, String> attributes) {
        emitMetric(name, value, attributes);
    }

    public void histogram(String name, double value) { histogram(name, value, null); }
    public void histogram(String name, double value, Map<String, String> attributes) {
        emitMetric(name, value, attributes);
    }

    private void emitMetric(String name, double value, Map<String, String> attributes) {
        metrics.add(new MetricPoint(System.currentTimeMillis(), name, value, attributes == null ? Map.of() : attributes));
        maybeDrop();
    }

    public void span(String traceId, String spanId, String name, double durationMs, String status) {
        span(traceId, spanId, name, durationMs, status, this.service, null);
    }

    public void span(String traceId, String spanId, String name, double durationMs, String status, String service, String parentSpanId) {
        long now = System.currentTimeMillis();
        spans.add(new SpanRecord(traceId, spanId, parentSpanId == null ? "" : parentSpanId, service, name, now * 1_000_000L, Math.round(durationMs * 1_000_000L), status));
        maybeDrop();
    }

    public boolean flush() {
        if (closed) return false;
        java.util.List<LogRecord> l;
        java.util.List<MetricPoint> m;
        java.util.List<SpanRecord> s;
        synchronized (logs) {
            l = new java.util.ArrayList<>(logs); logs.clear();
            m = new java.util.ArrayList<>(metrics); metrics.clear();
            s = new java.util.ArrayList<>(spans); spans.clear();
        }
        if (l.isEmpty() && m.isEmpty() && s.isEmpty()) return true;
        StringBuilder sb = new StringBuilder(1024);
        sb.append("{\"resource_attrs\":{\"service.name\":\"").append(json(service)).append("\",\"service.version\":\"").append(json(version)).append("\"}");
        if (tenant != null && !tenant.isEmpty()) sb.append(",\"tenant_id\":\"").append(json(tenant)).append("\"");
        sb.append(",\"logs\":[");
        for (int i = 0; i < l.size(); i++) {
            if (i > 0) sb.append(",");
            LogRecord r = l.get(i);
            sb.append("{\"timestamp_ns\":").append(r.timestamp_ns)
              .append(",\"severity_text\":\"").append(json(r.severity)).append("\"")
              .append(",\"body\":\"").append(json(r.body)).append("\"");
            if (!r.attributes.isEmpty()) sb.append(",\"attributes\":").append(mapJson(r.attributes));
            sb.append("}");
        }
        sb.append("],\"metrics\":[");
        for (int i = 0; i < m.size(); i++) {
            if (i > 0) sb.append(",");
            MetricPoint p = m.get(i);
            sb.append("{\"timestamp\":").append(p.timestamp).append(",\"name\":\"").append(json(p.name)).append("\",\"value\":").append(p.value);
            if (!p.attributes.isEmpty()) sb.append(",\"attributes\":").append(mapJson(p.attributes));
            sb.append("}");
        }
        sb.append("],\"spans\":[");
        for (int i = 0; i < s.size(); i++) {
            if (i > 0) sb.append(",");
            SpanRecord r = s.get(i);
            sb.append("{\"trace_id\":\"").append(json(r.trace_id)).append("\",\"span_id\":\"").append(json(r.span_id))
              .append("\",\"parent_span_id\":\"").append(json(r.parent_span_id)).append("\"")
              .append(",\"service\":\"").append(json(r.service)).append("\",\"name\":\"").append(json(r.name)).append("\"")
              .append(",\"start_unix_nano\":").append(r.start_unix_nano)
              .append(",\"duration_ns\":").append(r.duration_ns)
              .append(",\"status\":\"").append(json(r.status)).append("\"")
              .append("}");
        }
        sb.append("]}");
        try {
            HttpRequest req = HttpRequest.newBuilder(URI.create(baseUrl + "/api/ingest/otlp"))
                .header("Content-Type", "application/json")
                .header("Authorization", "Bearer " + apiKey)
                .POST(HttpRequest.BodyPublishers.ofString(sb.toString()))
                .timeout(Duration.ofSeconds(5))
                .build();
            HttpResponse<String> r = http.send(req, HttpResponse.BodyHandlers.ofString());
            return r.statusCode() >= 200 && r.statusCode() < 300;
        } catch (Exception e) {
            return false;
        }
    }

    private void flushSafe() { try { flush(); } catch (Throwable ignored) {} }

    private void maybeDrop() {
        int total = logs.size() + metrics.size() + spans.size();
        if (total > maxBuffer) {
            synchronized (logs) {
                while (logs.size() + metrics.size() + spans.size() > maxBuffer) {
                    if (!logs.isEmpty()) logs.poll();
                    else if (!metrics.isEmpty()) metrics.poll();
                    else if (!spans.isEmpty()) spans.poll();
                    else break;
                }
            }
        }
    }

    @Override public void close() {
        closed = true;
        flusher.shutdownNow();
        flush();
    }

    private static String json(String s) {
        if (s == null) return "";
        StringBuilder sb = new StringBuilder(s.length() + 2);
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '\\': sb.append("\\\\"); break;
                case '"': sb.append("\\\""); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                default:
                    if (c < 0x20) sb.append(String.format("\\u%04x", (int) c));
                    else sb.append(c);
            }
        }
        return sb.toString();
    }

    private static String mapJson(Map<String, String> m) {
        StringBuilder sb = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, String> e : m.entrySet()) {
            if (!first) sb.append(",");
            first = false;
            sb.append("\"").append(json(e.getKey())).append("\":\"").append(json(e.getValue())).append("\"");
        }
        sb.append("}");
        return sb.toString();
    }

    public static final class Builder {
        private String baseUrl = System.getenv().getOrDefault("DOG_BASE_URL", "http://localhost:18080");
        private String apiKey = System.getenv().getOrDefault("DOG_API_KEY", "");
        private String service = "java";
        private String version = "";
        private String tenant = System.getenv().getOrDefault("DOG_TENANT", "");
        private int maxBuffer = 10_000;
        private long flushIntervalMs = 1000;
        public Builder baseUrl(String v) { this.baseUrl = v; return this; }
        public Builder apiKey(String v) { this.apiKey = v; return this; }
        public Builder service(String v) { this.service = v; return this; }
        public Builder version(String v) { this.version = v; return this; }
        public Builder tenant(String v) { this.tenant = v; return this; }
        public Builder maxBuffer(int v) { this.maxBuffer = v; return this; }
        public Builder flushIntervalMs(long v) { this.flushIntervalMs = v; return this; }
        public Client build() {
            return new Client(baseUrl, apiKey, service, version, tenant, maxBuffer, flushIntervalMs);
        }
    }

    private record LogRecord(long timestamp_ns, String severity, String body, Map<String, String> attributes) {}
    private record MetricPoint(long timestamp, String name, double value, Map<String, String> attributes) {}
    private record SpanRecord(String trace_id, String span_id, String parent_span_id, String service,
                             String name, long start_unix_nano, long duration_ns, String status) {}
}
