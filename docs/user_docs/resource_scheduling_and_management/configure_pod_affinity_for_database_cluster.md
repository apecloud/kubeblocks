# Configure pods affinity for database clusters
Affinity controls the selection logic of pods allocation on nodes. By a reasonable allocation of kubernetes pods on different nodes, the business availability, resource usage rate, and stability are improved. 
## Default configuration
No affinity parameters required.
## Spread evenly as much as possible
Perform the following steps when you have resource to deploy but try to spread existing pods evenly as much as possible.
### Before you start:
Make sure all your Kubernetes nodes are labeled with `topologyKey`. 
Example: the following failure domain is specified as a node, and the node must be labeled with the `kubernetes.io/hostname label`. If there is no label, you can check [Add label to a node](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/#add-a-label-to-a-node) the to add a label to a node.
### API parameters
- Specify `topologyKeys` value as `kubernetes.io/hostname`
- Specify `podAntiAffinity` as `Preferred`
- If there is no specified node, don't specify `nodeLabels`.
``` 
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql-cluster
  namespace: default
spec:
  clusterDefinitionRef: wesql-clusterdefinition
  clusterVersionRef: wesql-clusterversion-8.0.29
  affinity:
    podAntiAffinity: preferred
    topologyKeys:
    - kubernetes.io/hostname
  components:
    ......
...... 
```
### Results
  - Pods spread evenly according to the specified topologyKey. In the above code example, `kubernetes.io/hostname` is used.
## Forced spread evenly
For cross available zone deployment and failure domain is specified as available zone. Configure it as forced antiaffinity.
Note: when there is no enough resource, the scheduling is failed.
### Before you start
- Label nodes with `topology.kubernetes.io/zone` and make sure values are different from one other.
e.g. for three nodes to deploy in different available zones, label three nodes with `us-east-1a` `us-east-1b` and `us-east-1c` separately before deploying.
### API parameters
- Specify `topologyKeys` value as `topology.kubernetes.io/zone​`
- Specify  `podAntiAffinity` value as `Required​`
- If there is no specified node, don't specify `nodeLabels`.
```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql-cluster
  namespace: default
spec:
  clusterDefinitionRef: wesql-clusterdefinition
  clusterVersionRef: wesql-clusterversion-8.0.29
  affinity:
    podAntiAffinity: Required
    topologyKeys:
    - topology.kubernetes.io/zone
  components:
    ......
......
```
### Results
- Three pods are deployed in three different available zones. 
## Deploy pods in specified nodes
You can restrict your pod to be deployed in any pre-defined nodes. For example, if you want to depoly nodes only on available zone us-east-1c, label `topology.kubernetes.io/zone` of some nodes with a certain available zone as `us-east-1c`, and others don't.
Note: if there is no enough resource, the scheduling is failed.
## API parameters
Specify `nodeLabels` 
```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql-cluster
  namespace: default
spec:
  clusterDefinitionRef: wesql-clusterdefinition
  clusterVersionRef: wesql-clusterversion-8.0.29
  affinity:
    nodeLabels:
    - topology.kubernetes.io/zone: us-east-1c
  components:
    ......
......
```
### Results
Pods are spread on the nodes with `topology.kubernetes.io/zone` value as `us-east-1c`, not others.