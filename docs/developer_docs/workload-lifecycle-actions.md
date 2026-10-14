# Workload lifecycle actions

The workload `Action` type is an alias of the ComponentDefinition `Action` type.
`Switchover`, `Reconfigure`, `MemberJoin`, `MemberLeave`, `DataDump`, and `DataLoad`
have the same purpose in both APIs. Data copying uses the existing init worker,
with dump output on stdout and load input on stdin.

The differences below apply to the workload execution enabled by `MemberJoin`,
`MemberLeave`, `DataDump`, or `DataLoad`. Existing workloads that only configure
`Switchover` or `Reconfigure` keep their existing behavior.

| Area | Workload behavior | Current Component path |
| --- | --- | --- |
| Preconditions | Join, leave, switchover, dump, and load accept an unset precondition or `Immediately`. The workload controller observes runtime readiness. | The action client also supports runtime, Component, and Cluster readiness preconditions. |
| Member join | Waits for Pod availability, including `minReadySeconds`, and completion of tracked data loading. | Uses the action client and its configured preconditions; data-copy progress is not an additional join gate. |
| Data source | Uses the shared target selector and selects one available source. It excludes the target, deleting or inactive Pods, known incomplete data, and Pods without a streaming endpoint. | The Component data-copy path excludes replicas still being provisioned before applying the selector. |
| Data configuration | Requires `DataDump`, `DataLoad`, `DataVolume`, and a `Worker` image together. `DataVolume` must identify a PVC-backed Pod volume. | The Component builds worker configuration from its definition and instance template. |
| Execution image | Uses the configured agent and worker images. An action's explicit `Exec.Image` must match the image executing that action. `Exec.Container` supplies its environment, mounts, security context, and working directory. | The Component can use a custom action image and copy the agent binary into it. |
| Completion | The worker records successful loading on the target PVC. A rebuilt worker reads this result before loading again. Its ServiceAccount needs permission to read and update that PVC. | Existing data tasks report events that update the Component's scaling annotations. |
| Variables | The caller supplies `TemplateVars`; the workload passes them to member actions and data tasks. | The Component resolves variables from its definition; its existing data tasks do not carry this explicit parameter map. |
| Updates | The workload API does not make lifecycle fields immutable. Changes remain subject to the existing workload update policy. | The ComponentDefinition controller validates its spec as immutable, including lifecycle actions. |

Workload-owned join, leave, switchover, dump, and load actions require
`NonBlocking=false`, consistent with the ComponentDefinition built-in actions.
The existing workload `Reconfigure` behavior is unchanged.

Action definitions are in [the workload API](../../apis/workloads/v1/instanceset_types.go)
and [the ComponentDefinition API](../../apis/apps/v1/componentdefinition_types.go).
