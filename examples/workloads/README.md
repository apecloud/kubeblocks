# Workload-owned data copying

This example creates an InstanceSet directly, with an independently bound ServiceAccount. It requires the controller and tools image built from the lifecycle implementation, the generated InstanceSet CRD, and a default StorageClass. Run the controller with `--apps=false --workloads=true`, or set Helm `controllers.apps.enabled=false` and `controllers.workloads.enabled=true`.

Build the tools image from this checkout and make it available to the Kubernetes nodes:

```sh
docker build -f docker/Dockerfile-tools -t kubeblocks-tools:its-lifecycle .
# For a local minikube cluster:
minikube image load kubeblocks-tools:its-lifecycle
```

For another cluster, publish the image to your registry and set `spec.lifecycleActions.worker.image` in `data-copy.yaml` to that image. The image contains `/bin/kbagent`, `/bin/sh`, and `cat`. The worker command defaults to `/bin/kbagent`. A custom image can instead use a shared binary, but its Pod template must configure the binary-copy init container and shared volume mount before the data worker runs.

Apply the CRD and the example into one namespace:

```sh
kubectl apply -f config/crd/bases/workloads.kubeblocks.io_instancesets.yaml
kubectl apply -f config/crd/bases/workloads.kubeblocks.io_instances.yaml
kubectl create namespace data-copy-example
kubectl -n data-copy-example apply -f examples/workloads/data-load-rbac.yaml
kubectl -n data-copy-example apply -f examples/workloads/data-copy.yaml
until kubectl -n data-copy-example get pod data-copy-0 >/dev/null 2>&1; do sleep 1; done
kubectl -n data-copy-example wait --for=condition=Ready pod/data-copy-0 --timeout=180s
kubectl -n data-copy-example exec data-copy-0 -c store -- cat /data/payload
```

The initial instance writes `data-copy-0:seed` into its PVC. Initial bootstrap has no tracked DataLoaded result and does not wait for a completion annotation. Add one replica after that Pod is ready:

```sh
kubectl -n data-copy-example scale instanceset data-copy --replicas=2
until kubectl -n data-copy-example get pod data-copy-1 >/dev/null 2>&1; do sleep 1; done
kubectl -n data-copy-example wait --for=condition=Ready pod/data-copy-1 --timeout=180s
kubectl -n data-copy-example exec data-copy-1 -c store -- cat /data/payload
kubectl -n data-copy-example get pvc data-data-copy-1 \
  -o go-template='{{ index .metadata.annotations "workloads.kubeblocks.io/data-loaded" }}{{ "\n" }}'
kubectl -n data-copy-example get instanceset data-copy \
  -o jsonpath='{.status.instanceStatus[?(@.podName=="data-copy-1")].dataLoaded}{"\n"}'
```

The copied payload must still be `data-copy-0:seed`, and both the PVC annotation and the target instance's DataLoaded status must be `true`. The controller injects DataDump into the source kbagent server. The target init worker streams its output into DataLoad stdin, then records completion on the target PVC before the application starts.

Check that replacing the target Pod retains its data:

```sh
kubectl -n data-copy-example delete pod data-copy-1 --wait=true
# Wait for the replacement Pod to appear before waiting for Ready.
until kubectl -n data-copy-example get pod data-copy-1 >/dev/null 2>&1; do sleep 1; done
kubectl -n data-copy-example wait --for=condition=Ready pod/data-copy-1 --timeout=180s
kubectl -n data-copy-example exec data-copy-1 -c store -- cat /data/payload
```

The replacement reads the retained PVC completion annotation and skips loading. The independent startup check remains after one-time task cleanup. A PVC without that result, unavailable API credentials, or missing read permission prevents worker success. A failed result write leaves the worker running and retrying the write without reloading data. The Role in `data-load-rbac.yaml` grants PVC `get` and `update` in this namespace; the Pod explicitly selects the ServiceAccount and mounts API credentials.

To run through Instance instead of direct Pods, set `spec.enableInstanceAPI: true` before the first apply. For a remote ITS2 placement, install the image and apply `data-load-rbac.yaml` in the runtime cluster and namespace. The worker uses that cluster's API credentials.

This file-copy fixture demonstrates the data worker and persistence contract. It does not configure engine membership actions. The example retains PVCs after deletion; remove the namespace when the example data is no longer needed:

```sh
kubectl delete namespace data-copy-example
```
