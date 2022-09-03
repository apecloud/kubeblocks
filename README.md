# Kubelego

![Release status](https://github.com/apecloud/kubeblocks/-/badges/release.svg)
![Pipeline](https://github.com/apecloud/kubeblocks/badges/first-demo/pipeline.svg)
![Codecoverage](https://github.com/apecloud/kubeblockst/badges/first-demo/coverage.svg)


## Overview

Kubelego Core Driver Controller Manager.

Features/Enhancement:
- [Operator developer guides](OPERATOR_DEVELOPER.md)
- Fast Multi-arch build docker images
- Helm Chart for deployment
  - Horizontal Pod Auto-Scaler (HPA)
  - Prometheus Service Monitor
  - RBAC
  - Pod Disruption Budget (PDB)
  - CRDs installation
  - Self-signed certificates for admission webhook configurations

## Quick start

```shell
$ make help

Usage:
  make <target>

General
  help             Display this help.

Development
  manifests        Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
  generate         Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
  fmt              Run go fmt against code.
  vet              Run go vet against code.
  cue-fmt          Run cue fmt against code.
  cue-vet          Run cue vet against code.
  lint             Run golangci-lint against code.
  staticcheck      Run staticcheck against code. 
  build-checks     Run build checks.
  mod-download     Run go mod download against go modules.
  mod-vendor       Run go mod tidy->vendor->verify against go modules.
  test             Run tests.
  test-webhook-enabled  Run tests with webhooks enabled.
  cover-report     Generate cover.html from cover.out
  goimports        Run goimports against code.

CLI
  dbctl            Build bin/dbctl CLI.
  clean-dbctl      Clean bin/dbctl* CLI tools.
  docker-build-cli  Build docker image with the dbctl.

Operator Controller Manager
  manager          Build manager binary.
  webhook-cert     Create root CA certificates for admission webhooks testing.
  run              Run a controller from your host.
  run-delve        Run Delve debugger.
  docker-build     Build docker image with the manager.
  docker-push      Push docker image with the manager.

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
  helm-push        Do helm package and push.

WeSQL Cluster Helm Chart Tasks
  bump-chart-ver-wqsql-cluster  Bump WeSQL Clsuter helm chart version.
  helm-package-wqsql-cluster  Do WeSQL Clsuter helm package.
  helm-push-wqsql-cluster  Do WeSQL Clsuter helm package and push.

Build Dependencies
  kustomize        Download kustomize locally if necessary.
  controller-gen   Download controller-gen locally if necessary.
  envtest          Download envtest-setup locally if necessary.
  install-docker-buildx  Create `docker buildx` builder.
  golangci         Download golangci-lint locally if necessary.
  staticchecktool  Download staticcheck locally if necessary.
  goimportstool    Download goimports locally if necessary.
  cuetool          Download cue locally if necessary.
  helmtool         Download helm locally if necessary.
  brew-install-prerequisite  Use `brew install` to install required dependencies. 
```


# TODO
- [ ] CI/CD integration
