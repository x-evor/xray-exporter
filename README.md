# xray-exporter

`xray-exporter` is the v1 translation layer for the Cloud Network Billing & Control Plane.

It polls raw Xray traffic counters, enriches them with account identity labels from
`accounts.svc.plus`, exposes Prometheus metrics, and publishes the latest normalized
snapshot for `billing-service`.

## Endpoints

- `GET /metrics`
- `GET /healthz`
- `GET /v1/snapshots/latest`

## Environment

- `XRAY_STATS_URL`
- `XRAY_STATS_TOKEN`
- `ACCOUNTS_BASE_URL`
- `INTERNAL_SERVICE_TOKEN`
- `EXPORTER_NODE_ID`
- `EXPORTER_ENV`
- `SCRAPE_INTERVAL`
- `LISTEN_ADDR`
