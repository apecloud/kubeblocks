# Installation guide

## Test/Dev

```bash
helm install lb .
```

## Production

Use `values-production.yaml` to override default resources.

```bash
helm install lb --values values.yaml --values values-production.yaml .
```