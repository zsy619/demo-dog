# Kubernetes deployment (Round 22.6)

Two ways to ship the collector:

## Plain manifests (k8s/)

kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml

Includes:

- Deployment (replicas=2, securityContext hardened, liveness +
  readiness + startup probes on /api/health and /api/probe)
- Service (ClusterIP, two ports: 80 = API, 9090 = metrics)
- HorizontalPodAutoscaler (CPU 70%, memory 80%)
- PodDisruptionBudget (minAvailable=1)
- PersistentVolumeClaim (1Gi for the snapshot file)
- ConfigMap + Secret + Ingress (optional)
- ServiceMonitor (optional, when prometheus-operator is installed)

## Helm chart (helm/dog-collector/)

helm install dog ./helm/dog-collector --namespace observability --create-namespace --set auth.apiKeys=... --set auth.pprofToken=...

Tunable values:

- replicaCount, hpa.{min,max}Replicas
- config.{workers,queue,rateLimit,rateBurst,selfTrace}
- persistence.{size,storageClass,accessModes}
- ingress.{enabled,hosts,tls}
- serviceMonitor.enabled (requires prometheus-operator)

## Health probes

- /api/health — liveness
- /api/probe — readiness, no auth
- /metrics — Prometheus scrape endpoint

## Snapshot persistence

The collector writes its in-memory state to /var/lib/dog-collector/state.snap
on graceful shutdown and reads it back at startup. The PVC keeps the
snapshot across pod restarts.
