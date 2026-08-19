# demo-dog Grafana integration

JSON dashboards that plug into any Grafana 10+ instance. The
dashboards target a Prometheus datasource whose query endpoint is
demo-dog's `/api/v1/query` PromQL-lite.

## Quickstart

### 1. Add the datasource

In Grafana UI: Configuration → Data sources → Add data source →
Prometheus → URL: `http://demo-dog-collector:8088`. Save & test.

### 2. (Optional) Provision via files

For automated deployments (Kubernetes, Docker, etc.):

```yaml
# /etc/grafana/provisioning/datasources/demo-dog.yaml
apiVersion: 1
datasources:
  - name: demo-dog
    type: prometheus
    access: proxy
    url: http://demo-dog-collector:8088
    isDefault: true
    jsonData:
      httpMethod: POST
      timeInterval: 30s
```

The shipped file at `provisioning/datasources/demo-dog.yaml` contains
the same configuration.

### 3. Provision the dashboards

```yaml
# /etc/grafana/provisioning/dashboards/demo-dog.yaml
apiVersion: 1
providers:
  - name: demo-dog
    folder: demo-dog
    type: file
    options:
      path: /var/lib/grafana/dashboards/demo-dog
```

Copy the JSON files from `demo-dog-overview.json` and
`demo-dog-prometheus.json` into `/var/lib/grafana/dashboards/demo-dog/`.

Grafana auto-reloads within 60 seconds.

## Compatible panels

Anything that speaks PromQL works:

| Panel | Works? | Notes |
|---|---|---|
| Time series | yes | standard |
| Stat / gauge | yes | use `scalar()` if needed |
| Bar gauge | yes | aggregate by label |
| Table | yes | instantiate columns |
| Heatmap | yes | use histogram_quantile |
| Logs | partial | demo-dog logs are reachable via `/api/ingest/recent`; bring a Loki datasource for full text search |
| Trace | yes | use the Jaeger datasource, or expose demo-dog's trace API |

## Pre-built dashboards

* **`demo-dog-overview.json`** — RPS / error rate / p99 latency
  per service, with annotations on deploys
* **`demo-dog-prometheus.json`** — Prometheus-style global view
  across all services

Both reference the `Prometheus` datasource, so once you wire up
`demo-dog` as Prometheus, the dashboards work without changes.

## Variables

Template variables let users pick which service to view. demo-dog
exposes them via `/api/v1/series`:

```
GET /api/v1/series?match[]={__name__="http.server.duration"}
```

In Grafana: Settings → Variables → New → Type: Query →
Data source: demo-dog → Query: `label_values(http_server_duration, service)`.

## Annotations

demo-dog can emit deploy annotations via a webhook. Configure
Grafana to pull from `https://your-deploy-system.com/api/annotations`
and demo-dog will appear in the timeline when a new release rolls
out.

## Alertmanager

demo-dog's SLO burn-rate rules (`/api/v1/rules`) are visible from
Grafana's Alerting → Alert rules panel. Grafana reads them like any
other Prometheus rules source. Set the rules URL to
`http://demo-dog-collector:8088/api/v1/rules` in the Grafana
Prometheus datasource config (this works because the response shape
matches the Prometheus rules API).
