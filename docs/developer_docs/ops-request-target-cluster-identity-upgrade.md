# OpsRequest target Cluster identity upgrade

Install the updated OpsRequest CRD before rolling out a controller that writes `status.targetClusterUID`.

Before the controller rollout, stop new OpsRequest submissions and drain every existing `Pending`, `Creating`, `Running`, or `Cancelling` request to a terminal phase. An active request without `status.targetClusterUID` cannot prove which Cluster object accepted it. The new controller aborts such a request with reason `TargetIdentityUnknown`; it does not infer the UID from an owner reference or a Cluster with the same name.

Check for active unbound requests before rollout:

```shell
kubectl get opsrequests.operations.kubeblocks.io -A -o json | \
  jq -r '.items[] | select((.status.phase == "Pending" or .status.phase == "Creating" or .status.phase == "Running" or .status.phase == "Cancelling") and ((.status.targetClusterUID // "") == "")) | [.metadata.namespace, .metadata.name, .spec.clusterName, .status.phase] | @tsv'
```

Resolve or finish every reported request under the old controller. If an OpsRequest bypassed its finalizer, inventory its Cluster queue entry and any task objects before rollout. Clean an owned task through its existing operation-specific path only when an immutable request UID proves ownership. For a name-only queue entry or task, first verify that no operation still owns active work, then use a bounded manual procedure with a resource-version or UID precondition. The new controller does not scan the namespace and remove queue entries or tasks by request name after the OpsRequest has disappeared.

Do not downgrade to a controller that ignores `status.targetClusterUID` while requests accepted by the new controller are active. Stop submissions and drain or cancel those requests according to their existing operation contracts before downgrade.

This change protects the common OpsRequest controller entry and its Cluster writes. Full target identity release also requires the Parameters and data-protection identity changes, owner checks for initialization and Switchover, and the selected HA retirement or owner-specific replacement work.
