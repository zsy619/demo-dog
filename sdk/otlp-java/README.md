# demo-dog Java SDK

Stdlib-only OTLP-style client. Requires JDK 11+.

```java
try (DemoDogClient c = DemoDogClient.builder()
    .baseUrl("http://localhost:18080")
    .apiKey(System.getenv("DOG_API_KEY"))
    .service("checkout")
    .tenant("acme")
    .build()) {
  c.log("INFO", "order placed");
  c.counter("orders.placed", 1.0);
  c.histogram("checkout.duration_ms", 78.5);
  c.span("trace-1", "span-1", "GET /checkout", 120, "ok");
}
```

Records are buffered in-memory and flushed on a scheduled thread
(every 1s by default). The buffer is bounded by `maxBuffer`
(10000 default).

## Build

```bash
javac -d build src/main/java/io/demodog/sdk/*.java
```

## Test

Cross-language coverage lives in `frontend/e2e/`. There is no
unit test suite in the SDK itself; it is small enough that
constructor shape + a smoke round-trip are sufficient.
