# Backup and restore for MySQL single node 
This section shows how to use kbcli to back up and restore a MySQL single-node instance.
Before you start
- Prepare a clean EKS cluster, and install ebs csi driver plug-in, with at least one node and the memory of each node is not less than 4GB.
- Install kubectl to ensure that you can connect to the EKS cluster 
- Install kbcli, quote

Steps:
1. Install kubeblocks and enable snapshot backup.
Install KubeBlocks and enable the snapshot controller plugin.
```
kbcli kubeblocks install --set snapshot-controller.enabled=true
```
Since your kubectl is already connected to the EKS cluster, this command installs the latest version of KubeBlocks in your EKS environment.

Verify the installation with the following command.
```
kubectl get pod
```

The pod with kubeblocks-snapshot-controller is shown. See the information below.
```
NAME                                              READY   STATUS             RESTARTS      AGE
kubeblocks-5c8b9d76d6-m984n                       1/1     Running            0             9m
kubeblocks-snapshot-controller-6b4f656c99-zgq7g   1/1     Running            0             9m
```
2. Configure EKS to support the snapshot function.
The backup is realized by the volume snapshot function, you need to configure EKS to support the snapshot function.
- Configure storage class of snapshot (the assigned ebs volume is gp3).
```
kubectl create -f - <<EOF
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: ebs-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: ebs.csi.aws.com
parameters:
  csi.storage.k8s.io/fstype: xfs
  type: gp3
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
EOF

kubectl patch sc/gp2 -p '{"metadata": {"annotations": {"storageclass.kubernetes.io/is-default-class": "false"}}}'
```
- Configure default snapshot volumesnapshot class
```
cat <<"EOF" > snapshot_class.yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-aws-vsc
  annotations:
    snapshot.storage.kubernetes.io/is-default-class: "true"
driver: ebs.csi.aws.com
deletionPolicy: Delete
EOF

kubectl create -f snapshot_class.yaml
```
