# `/test`

Additional external test apps and test data. Feel free to structure the `/test` directory anyway you want. For bigger projects it makes sense to have a data subdirectory. For example, you can have `/test/data` or `/test/testdata` if you need Go to ignore what's in that directory. Note that Go will also ignore directories or files that begin with "." or "_", so you have more flexibility in terms of how you name your test data directory.

Examples:

* https://github.com/openshift/origin/tree/master/test (test data is in the `/testdata` subdirectory)

## Helm Chart Rendering

Install Helm, then run `go test ./test/helm -count=1 -v` from the repository root.
These tests render the real chart without contacting a Kubernetes cluster. They
are skipped when Helm is not installed.

The chart tests cover datasafed registry overrides, legacy registry fallbacks,
and isolation from other rendered resources and image settings.
