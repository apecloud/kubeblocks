minikube delete
minikube start
make install
kubectl apply -f config/crd/bases/dataprotection.kubeblocks.io_backupschedules.yaml
helm install etcd deploy/etcd
helm install etcd-test deploy/etcd-cluster