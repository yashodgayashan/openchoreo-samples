# Latency Lab on OpenChoreo

OpenChoreo manifests for the [latency-lab](../../project-latency-lab) sample — a notes microservice with conditional latency / fault injection via query params. The shape mirrors
[`samples/from-source/projects/url-shortener`](https://github.com/openchoreo/openchoreo/tree/main/samples/from-source/projects/url-shortener).

## Layout

```
openchoreo/project-latency-lab/
├── project.yaml                       # Project: latency-lab
└── components/
    ├── postgres.yaml                  # lab-postgres          (deployment/service)
    ├── redis.yaml                     # lab-redis             (deployment/service)
    ├── api-service.yaml               # lab-api-service       (deployment/service)
    ├── analytics-service.yaml         # lab-analytics-service (deployment/service)
    └── frontend.yaml                  # lab-frontend          (deployment/web-application + 5xx alert)
```

Each component pairs a `Component` resource with a one-shot `WorkflowRun` that builds the image via the cluster `dockerfile-builder` workflow.

## Prerequisites

- An OpenChoreo cluster (control plane + observability plane).
- `kubectl` access.
- Source is pulled from `https://github.com/yashodgayashan/openchoreo-samples` on `main`. If you're working off a fork, swap the URL in `openchoreo/project-latency-lab/components/*.yaml` first.

## Deploy

Run from the repo root:

```bash
kubectl apply -f openchoreo/project-latency-lab/project.yaml
kubectl apply \
  -f openchoreo/project-latency-lab/components/postgres.yaml \
  -f openchoreo/project-latency-lab/components/redis.yaml \
  -f openchoreo/project-latency-lab/components/api-service.yaml \
  -f openchoreo/project-latency-lab/components/analytics-service.yaml \
  -f openchoreo/project-latency-lab/components/frontend.yaml
```

The frontend ships with an `observability-alert-rule` trait that fires when
more than 5 HTTP-500s appear in the logs within a minute — easy to trigger
on demand by hitting any endpoint with `?fail_rate=1`.

## Cleanup

```bash
kubectl delete -f openchoreo/project-latency-lab/project.yaml
```
