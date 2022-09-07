# TODO items

## DBaaS controllers

### Cluster CR controller
- [x] secondary resources finalizer
- [ ] CR delete handling
  - [x] delete secondary resources
  - [ ] CR spec.terminationPolicy handling
- [ ] managed resources handling
  - [ ] nodeGroup attached Service kind
  - [ ] deployment workloads
  - [ ] PDB
  - [ ] label handling:
    - [ ] deploy & sts workloads's labels and spec.template.metadata.labels (check https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)
- [ ] immutable spec properties handling (via validating webhook)
- [ ] CR status handling
- [ ] checked AppVersion CR status
- [ ] checked ClusterDefinition CR status
- [ ] CR update handling
  - [ ] PVC volume expansion (spec.components[].volumeClaimTemplates only works for initial statefulset creation)
  - [ ] spec.components[].serviceType
- [x] merge components from all the CRs

### ClusterDefinition CR controller
- [x] track changes and update associated CRs (Cluster, AppVersion) status
- [x] cannot delete ClusterDefinition CR if any referencing CRs (Cluster, AppVersion)

### AppVersion CR controller
- [x] immutable spec handling (via validating webhook)
- [x] CR status handling
- [x] cannot delete AppVersion CR if any referencing CRs (Cluster)

### Test
- [x] unit test