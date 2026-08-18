# SDK hardening (Round 22.4)

Four additions make the Go SDK safe for production traffic:

## 1. Bounded buffer

Per-stream caps with drop-oldest policy. Defaults are unbounded so
existing call sites are unaffected.

```go
import "github.com/zsy619/demo-dog/sdk/otlp-go/internal/buffer"

b := buffer.New("checkout", nil,
    buffer.WithCaps(50_000, 20_000, 50_000))  // log, metric, span
b.PushLog(buffer.LogRecord{Body: "..."})

dl, dm, ds := b.Stats()  // dropped counters
```

For SDK users:

```go
sdk, _ := otlp.New("http://localhost:18080",
    otlp.WithBufferCaps(50_000, 20_000, 50_000))
```

## 2. PII redactor

DefaultRedactor masks four secret classes:

- password=... / "password": "..."
- Bearer <token>
- Authorization: <scheme> <token>
- email addresses

Override with your own:

```go
otlp.WithRedactor(func(body string, attrs map[string]string) (string, map[string]string) {
    return strings.ReplaceAll(body, "internal-host", "REDACTED"), attrs
})
```

## 3. Tail-based sampler

```go
otlp.WithSampler(otlp.NewTailBasedSampler(250 /* ms */, 0.9))
```

Keeps every trace with an error or any span >= 250 ms. Probabilistic
drop for boring traces (default keep when no decision recorded).

## 4. Env config

```go
sdk, _ := otlp.FromEnv()  // reads DOG_ENDPOINT, DOG_API_KEY, DOG_TENANT
```

Defaults: http://localhost:18080. Missing values fall back to the
defaults rather than erroring.

## Tests

```bash
cd sdk/otlp-go
go test -race ./...
```
