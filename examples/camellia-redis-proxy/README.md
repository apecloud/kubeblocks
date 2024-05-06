# Camellia Redis Proxy

camellia-redis-proxy is a high-performance redis proxy developed using Netty4.

## Prerequisites

This example assumes that you have a Kubernetes cluster installed and running, and that you have installed the kubectl command line tool and helm somewhere in your path. Please see the [getting started](https://kubernetes.io/docs/setup/)  and [Installing Helm](https://helm.sh/docs/intro/install/) for installation instructions for your platform.

Also, this example requires kubeblocks installed and running. Here is the steps to install kubeblocks, please replace "0.9.0" with the version you want to use.
```bash
# Create dependent CRDs
kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/kubeblocks_crds.yaml
# If github is not accessible or very slow for you, please use following command instead
kubectl create -f https://jihulab.com/api/v4/projects/98723/packages/generic/kubeblocks/v0.9.0/kubeblocks_crds.yaml

# Add Helm repo 
helm repo add kubeblocks https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

# Update helm repo
helm repo update

# Install KubeBlocks
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version="0.9.0"
```
Enable camellia-redis-proxy
```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable
# Update helm repo
helm repo update

# Enable camellia-redis-proxy 
helm upgrade -i kb-addon-camellia-redis-proxy kubeblocks-addons/camellia-redis-proxy --version 0.9.0 -n kb-system
``` 

## Examples

### [Create](cluster.yaml) 
Create a camellia-redis-proxy cluster with specified cluster definition 
```bash
kubectl apply -f examples/camellia-redis-proxy/cluster.yaml
```

Starting from kubeblocks 0.9.0, we introduced a more flexible cluster creation method based on components, allowing customization of cluster topology, functionalities and scale according to specific requirements.
```bash
kubectl apply -f examples/camellia-redis-proxy/cluster-cmpd.yaml
```

### [Configure](configure.yaml)
Update camellia-cluster-proxy properties config.
```bash
kubectl get secret redisc-cluster-redis-account-default -ojsonpath='{.data.password}' | base64 -d
# yY224knB90

kubectl edit configmap camellia-cluster-proxy-application-config -n default
```
```yaml
apiVersion: v1
data:
  camellia-redis-proxy.properties: |-
##  This is dynamic configuration file for camellia-cluster-proxy
##  when `camellia-redis-proxy.transpond.custom.proxy-route-conf-updater-class-name` defined in application.yaml is `com.netease.nim.camellia.redis.proxy.route.DynamicConfProxyRouteConfUpdater`
##  `camellia-redis-proxy.client-auth-provider-class-name` defined in application.yaml is `com.netease.nim.camellia.redis.proxy.auth.DynamicConfClientAuthProvider`
                                     
##  provided for DynamicConfProxyRouteConfUpdater
#  1.default.route.conf=redis://@127.0.0.1:6379
#  2.default.route.conf=redis-cluster://@127.0.0.1:6380,127.0.0.1:6381,127.0.0.1:6382
#  3.default.route.conf={"type": "simple","operation": {"read": "redis://passwd123@127.0.0.1:6379","type": "rw_separate","write": "redis-sentinel://passwd2@127.0.0.1:6379,127.0.0.1:6378/master"}}

## provided for DynamicConfClientAuthProvider
#  password123.auth.conf=1|default
#  password456.auth.conf=2|default
#  password789.auth.conf=3|default
                                     
    1.default.route.conf=redis://yY224knB90@redisc-cluster-redis-redis.default.svc.cluster.local:6379
    
    password123.auth.conf=1|default

##  Another Configuration for multi-tenant proxies 
##  when `camellia-redis-proxy.transpond.custom.proxy-route-conf-updater-class-name` defined in application.yaml is `com.netease.nim.camellia.redis.proxy.route.MultiTenantProxyRouteConfUpdater`.
##  `camellia-redis-proxy.client-auth-provider-class-name` defined in application.yaml is `com.netease.nim.camellia.redis.proxy.auth.MultiTenantClientAuthProvider`

##  This is an array where each item represents a route, supporting multiple sets of routes.
#  multi.tenant.route.config=[{"name":"route1", "password": "passwd1", "route": "redis://passxx@127.0.0.1:16379"},{"name":"route2", "password": "passwd2", "route": "redis-cluster://@127.0.0.1:6380,127.0.0.1:6381,127.0.0.1:6382"},{"name":"route3", "password": "passwd3", "route": {"type": "simple","operation": {"read": "redis://passwd123@127.0.0.1:6379","type": "rw_separate","write": "redis-sentinel://passwd2@127.0.0.1:6379,127.0.0.1:6378/master"}}}]
  
    multi.tenant.route.config=[{"name":"route1","password":"password123","route":"redis://yY224knB90@redisc-cluster-redis-redis.default.svc.cluster.local:6379"}]
```

Configure parameters with the specified components in the cluster
```bash
kubectl apply -f examples/camellia-redis-proxy/configure.yaml
```


### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/camellia-redis-proxy/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/camellia-redis-proxy/verticalscale.yaml
```

### [Expand volume](volumeexpand.yaml)
Increase size of volume storage with the specified components in the cluster
```bash
kubectl apply -f examples/camellia-redis-proxy/volumeexpand.yaml
```

### [Restart](restart.yaml)
Restart the specified components in the cluster
```bash
kubectl apply -f examples/camellia-redis-proxy/restart.yaml
```

### [Stop](stop.yaml)
Stop the cluster and release all the pods of the cluster, but the storage will be reserved
```bash
kubectl apply -f examples/camellia-redis-proxy/stop.yaml
```

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/camellia-redis-proxy/start.yaml
```

### Expose
Expose a cluster with a new endpoint
#### [Enable](expose-enable.yaml)
```bash
kubectl apply -f examples/camellia-redis-proxy/expose-enable.yaml
```

#### [Disable](expose-disable.yaml)
```bash
kubectl apply -f examples/camellia-redis-proxy/expose-disable.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster camellia-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster camellia-cluster

kubectl delete cluster redisc-cluster
```
