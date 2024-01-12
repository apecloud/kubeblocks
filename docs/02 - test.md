## Test
`KubeBlocks` uses make for a variety of test actions.

### Envtest
Setting up a local control plane and test all packages:
```shell
make test
```
Test specific packages:

```shell
# Directory `controllers` contains many packages, it will build and run each package individually.
make test TEST_PACKAGES=./controllers/...

# Test single package
make test-delve TEST_PACKAGES=./controllers/apps/...
```

### Use existing Kubernetes cluster
```shell
make test-current-ctx
```
Instead of setting up a local control plane, this command will point to the control plane of an existing cluster by `$HOME/.kube/config`.

### Check test code coverage

```shell
make test cover-report
```
This command will test all packages and generate a coverage report file `cover.html`, and open by your default browser automatically.

