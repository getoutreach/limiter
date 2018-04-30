# Introduction

This controller dynamically injects (and subsequently reconciles) `LimitRange` resources in Kubernetes namespaces.

# Installation
This controller is expected to run within all outreach kubernetes clusters via Docker, but can also be run outside
of clusters during development.
To do so, simply point the `--kubeconfig` flag at a valid kube config file, e.g:
```bash
go get github.com/getoutreach/limiter
$GOPATH/bin/limiter --kubeconfig $HOME/.kube/config
```

# Options

| Option | Default | Description |
| ------ | ------- | ----------- |
| `--kubeconfig` | | Path to a kubeconfig.<br>Only required if out-of-cluster. |
| `--master` | | The (override) address of the Kubernetes API server.<br>Only required if out-of-cluster. |
| `--container.limit.cpu` | | Default container CPU limit. |
| `--container.limit.memory` | "100Mi" | Default container memory limit. |
| `--container.request.cpu` | "100m" |  Default container CPU request. |
| `--container.request.memory` | "10Mi" | Default container memory request. |
| `--container.max.cpu` | | Maximum container CPU limit. |
| `--container.max.memory` | | Maximum container memory limit. |
| `--container.min.cpu` | | Minimum container CPU request. |
| `--container.min.memory` | | Minimum container memory request. |
| `--pod.min.cpu` | | Minimum pod CPU request. |
| `--pod.max.cpu` | | Maximum pod CPU limit. |
| `--pod.min.memory` | | Minimum pod memory request. |
| `--pod.max.memory` | | Maximum pod memory limit. |
| `--interval` | "30s" | Sync/loop Interval. |
| `--workers` | 2 | The number of worker goroutines to spawn. |
