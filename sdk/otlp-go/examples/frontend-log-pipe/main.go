// frontend-log-pipe — 演示前端 SDK 怎么借 Go 服务中转 telemetry。
//
// 场景:浏览器内 React/Next SPA 把结构化日志 + 指标 POST 到 Go 反代;
// 反代用 otlp SDK 汇总后批量转给 DOG collector。这样:
//   1. 后端 Go 服务也共用同一个 SDK,便于统一配置 + 标签;
//   2. 浏览器只走一个聚合入口,降低 collector 的 QPS;
//   3. 后端还可以补全 user_id / session_id 这些前端没有的信息。
//
//   go run ./examples/frontend-log-pipe
//   # 然后用 curl 模拟浏览器上报:
//   curl -X POST http://localhost:8083/api/ingest/frontend \
//     -H "Content-Type: application/json" \
//     -d '{"logs":[{"level":"info","message":"click buy"}],"metrics":[{"name":"checkout.clicks","value":1,"labels":{"page":"cart"}}]}'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

type frontendEnvelope struct {
	Logs    []frontendLog    `json:"logs"`
	Metrics []frontendMetric `json:"metrics"`
	UserID  string           `json:"user_id,omitempty"`
	Page    string           `json:"page,omitempty"`
}

type frontendLog struct {
	Level   string            `json:"level"`
	Message string            `json:"message"`
	At      int64             `json:"at_ms"`
	KVs     map[string]string `json:"kvs,omitempty"`
}

type frontendMetric struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels,omitempty"`
}

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("frontend-pipe"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithDeploymentEnvironment("web"),
		otlp.WithFlushInterval(1*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ingest/frontend", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var env frontendEnvelope
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			http.Error(w, "bad envelope: "+err.Error(), 400)
			return
		}

		for i := range env.Logs {
			sdk.Log(r.Context(),
				otlp.Severity(env.Logs[i].Level),
				env.Logs[i].Message,
				mergeKVs(
					env.Logs[i].KVs,
					map[string]string{
						"page":    env.Page,
						"user_id": env.UserID,
					},
				)...,
			)
		}
		for _, m := range env.Metrics {
			kvs := []otlp.KV{
				otlp.String("page", env.Page),
				otlp.String("user_id", env.UserID),
			}
			for k, v := range m.Labels {
				kvs = append(kvs, otlp.String(k, v))
			}
			sdk.Counter(r.Context(), m.Name, m.Value, kvs...)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{
			"accepted_logs":    len(env.Logs),
			"accepted_metrics": len(env.Metrics),
		})
	})

	log.Printf("frontend-log-pipe listening on :8083, exporting to %s", endpoint)
	log.Fatal(http.ListenAndServe(":8083", mux))
}

func mergeKVs(base, overlay map[string]string) []otlp.KV {
	out := make([]otlp.KV, 0, len(base)+len(overlay))
	for k, v := range base {
		out = append(out, otlp.String(k, v))
	}
	for k, v := range overlay {
		out = append(out, otlp.String(k, v))
	}
	return out
}

var _ = fmt.Println
