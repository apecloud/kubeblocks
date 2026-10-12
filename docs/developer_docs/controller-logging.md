# Controller logging

Controller reconciliation logs include `controllerType` and `resource`
(namespace/name). Failed reconciliation is logged at error level. Expected
conflicts and waits, reconciliation start/completion, transformer execution,
and workload reconciler decisions are logged at verbosity 1 (debug). Completion
records include `duration`, `requeue`, and `requeueAfter`.

Cluster, Component, Rollout and kubebuilderx resource operations log `action`,
`kind`, `namespace`, and `name`. Successful operations are visible at info level;
optimistic lock conflicts are debug messages. Transformer and workload
reconciler logs identify the concrete step so an error can be traced to its
stage. A transformer requesting a delayed retry can be followed by other steps;
this is expected because the partial plan still needs to execute.

To see normal reconciliation and waiting details, set the Helm value
`loggerSettings.level: debug` or pass `--zap-log-level=debug` to the controller
process. The default info level shows resource operations and failures without
logging every idle reconciliation. Filter by the resource namespace/name and
then by `transformer` or `reconciler` to locate the failing step.

Keep logs structured. Log resource identity and outcomes rather than object
bodies, Secret data, rendered configuration, or lifecycle action arguments.
