# KubeBlocks

[![Build status](https://github.com/apecloud/kubeblocks/workflows/CICD-PUSH/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/cicd-push.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/apecloud/kubeblocks)](https://goreportcard.com/report/github.com/apecloud/kubeblocks)
[![Docker Pulls](https://img.shields.io/docker/pulls/apecloud/kubeblocks)](https://hub.docker.com/r/apecloud/kubeblocks)
[![codecov](https://codecov.io/gh/apecloud/kubeblocks/branch/main/graph/badge.svg?token=GEH4I1C80Y)](https://codecov.io/gh/apecloud/kubeblocks)
[![LICENSE](https://img.shields.io/github/license/apecloud/kubeblocks.svg?style=flat-square)](/LICENSE)
[![Releases](https://img.shields.io/github/release/apecloud/kubeblocks/all.svg?style=flat-square)](https://github.com/apecloud/kubeblocks/releases)
[![TODOs](https://img.shields.io/endpoint?url=https://api.tickgit.com/badge?repo=github.com/apecloud/kubeblocks)](https://www.tickgit.com/browse?repo=github.com/apecloud/kubeblocks)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/apecloud)](https://artifacthub.io/packages/search?repo=apecloud)

## Overview

KubeBlocks Controller Manager.

Features/Enhancement:
- [Operator developer guides](https://book.kubebuilder.io/)
- Fast Multi-arch build docker images
- Helm Chart for deployment
  - Horizontal Pod Auto-Scaler (HPA)
  - Prometheus Service Monitor
  - RBAC
  - Pod Disruption Budget (PDB)
  - CRDs installation
  - Self-signed certificates for admission webhook configurations
- [Configuration template guides](docs/configuration/configuration_tpl_developer.md)

## Quick start

```shell
$ make help

Usage:
  make <target>

General
  help             Display this help.
  all              Make all cmd binaries.

Development
  manifests        Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
  generate         Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
  fmt              Run go fmt against code.
  vet              Run go vet against code.
  cue-fmt          Run cue fmt against code.
  cue-vet          Run cue vet against code.
  lint             Run golangci-lint against code.
  staticcheck      Run staticcheck against code. 
  loggercheck      Run loggercheck against code.
  build-checks     Run build checks.
  mod-download     Run go mod download against go modules.
  mod-vendor       Run go mod tidy->vendor->verify against go modules.
  ctrl-test-current-ctx  Run operator controller tests with current $KUBECONFIG context
  test             Run tests.
  test-webhook-enabled  Run tests with webhooks enabled.
  cover-report     Generate cover.html from cover.out
  goimports        Run goimports against code.

CLI
  dbctl            Build bin/dbctl CLI.
  clean-dbctl      Clean bin/dbctl* CLI tools.

Operator Controller Manager
  manager          Build manager binary.
  webhook-cert     Create root CA certificates for admission webhooks testing.
  run              Run a controller from your host.
  run-delve        Run Delve debugger.

Deployment
  install          Install CRDs into the K8s cluster specified in ~/.kube/config.
  uninstall        Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
  deploy           Deploy controller to the K8s cluster specified in ~/.kube/config.
  dry-run          Dry-run deploy job.
  undeploy         Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.

CI
  ci-test-pre      Prepare CI test environment.
  ci-test          Run CI tests.

Contributor
  reviewable       Run code checks to proceed with PR reviews.
  check-diff       Run git code diff checker.

Helm Chart Tasks
  bump-chart-ver   Bump helm chart version.
  helm-package     Do helm package.

WeSQL Cluster Helm Chart Tasks
  bump-chart-ver-wqsql-cluster  Bump WeSQL Cluster helm chart version.
  helm-package-wqsql-cluster  Do WeSQL Cluster helm package.

Build Dependencies
  kustomize        Download kustomize locally if necessary.
  controller-gen   Download controller-gen locally if necessary.
  envtest          Download envtest-setup locally if necessary.
  install-docker-buildx  Create `docker buildx` builder.
  golangci         Download golangci-lint locally if necessary.
  staticchecktool  Download staticcheck locally if necessary.
  loggerchecktool  Download loggercheck locally if necessary.
  goimportstool    Download goimports locally if necessary.
  cuetool          Download cue locally if necessary.
  helmtool         Download helm locally if necessary.
  oras             Download ORAS locally if necessary.
  minikube         Download minikube locally if necessary.
  brew-install-prerequisite  Use `brew install` to install required dependencies. 

Minikube
  minikube-start   Start minikube cluster.
  minikube-delete  Delete minikube cluster. 

Docker containers 
  build-dev-container  Build dev container image.
  push-dev-container  Push dev container image.
  build-cli-container  Build dbctl CLI container image.
  push-cli-container  Push dbctl CLI container image.
  build-manager-container  Build Operator manager container image.
  push-manager-container  Push Operator manager container image.
```
