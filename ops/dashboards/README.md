# demo-dog Grafana dashboards

JSON dashboards that plug into any Grafana 10+ instance. The
dashboards target a Prometheus datasource whose query endpoint is
demo-dog's `/api/v1/query` PromQL-lite.

## Files

* `demo-dog-overview.json` — the main overview dashboard.
  Panels: request rate, active services, error rate, latency
  percentiles, log volume per severity, top errored endpoints.
* `demo-dog-prometheus.json` — bridge dashboard for teams that
  already run Prometheus. Same panels, slightly different
  queries, includes `ingest_requests_total` and `demo_dog_*`
  metrics that demo-dog exposes.

## Setup

1. Add demo-dog as a Prometheus datasource in Grafana.
   * URL: `http://localhost:18080/api/v1/query`
   * Type: `Prometheus`
   * Access: `Server`
2. In Grafana, **Dashboards → Import**, paste the JSON from
   one of the files in this directory.
3. Select the datasource from step 1 when prompted.

The dashboards are templated on `service` and `datasource` so
they work for any tenant that exposes Prometheus-style labels.
