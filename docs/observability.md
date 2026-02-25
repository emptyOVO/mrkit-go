# Runtime Observability (M3)

This document describes the online metrics endpoint and minimal Grafana setup.

## Metrics Endpoint

`master` exposes a Prometheus-format endpoint:

- default: `http://127.0.0.1:<master_port+1000>/metrics`
- example: master `:10000` -> metrics `:11000`
- override: set `MR_METRICS_ADDR` (for example `:2112`)

Quick check:

```bash
curl -s http://127.0.0.1:11000/metrics | sed -n '1,80p'
```

## Core Metrics

- `task_total{stage,result}`: task lifecycle counter (`submitted|success|failed`)
- `task_retry_total{stage}`: task retry counter
- `worker_alive`: alive worker gauge from registry lease view
- `stage_duration_seconds{stage}`: last stage duration gauge in seconds

## Prometheus Scrape Example

```yaml
scrape_configs:
  - job_name: mrkit-master
    static_configs:
      - targets: ["127.0.0.1:11000"]
```

## Minimal Grafana Dashboard

Import dashboard JSON:

- `dashboards/mrkit-observability-minimal.json`

Included panels:

- task throughput: `sum(rate(task_total{result="success"}[1m])) by (stage)`
- worker alive gauge: `worker_alive`
- stage duration: `stage_duration_seconds`
- retry counter: `task_retry_total`
