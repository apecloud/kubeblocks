#Connect database in production environment
In the production environment, it is normal to connect database with CLI and SDK clients. There are three scenarios.
- Scenario 1: Client1 and the database are in the same Kubernetes cluster. To connect client1 and database, see Procedure 3.
- Scenario 2: Client2 is outside the Kubernetes cluster, but it is in the same VPC as the database. To connect client2 and database, see Procedure 5 or 6.
- Scenario 3: Client3 and the database are in different VPCs, such as other VPCs or the public network. To connect client3 and database, see Procedure 4.
See the figure below to get a clear image of the network location.
##Procedure 3.  To connect database in the same Kubernetes cluster.
 you can connect with database ClusterIP or domain name. To check the database address, use ```kbcli cluster describe ${cluster-name}```.

```
kbcli cluster describe x
Name: x         Created Time: Mar 01,2023 11:45 UTC+0800
NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
default     apecloud-mysql       ac-mysql-8.0.30   Running   Delete

Endpoints:
COMPONENT   MODE        INTERNAL                                 EXTERNAL
x           ReadWrite   x-mysql.default.svc.cluster.local:3306   <none>

Topology:
COMPONENT   INSTANCE    ROLE     STATUS    AZ                NODE                                                       CREATED-TIME
mysql       x-mysql-0   leader   Running   cn-northwest-1b   ip-10-0-2-184.cn-northwest-1.compute.internal/10.0.2.184   Mar 01,2023 11:45 UTC+0800

Resources Allocation:
COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
mysql       false       1 / 1                1Gi / 1Gi               data:10Gi      <none>

Images:
COMPONENT   TYPE    IMAGE
mysql       mysql   registry.cn-hangzhou.aliyuncs.com/apecloud/apecloud-mysql-server:8.0.30-5.alpha2.20230105.gd6b8719.2

Events(last 5 warnings, see more:kbcli cluster list-events -n default x):
TIME   TYPE   REASON   OBJECT   MESSAGE
```
##Procedure 4. To connect database with clients in other VPCs or public networks
You can enable the External LoadBalancer of the cloud vendor.
>Note: The following command will create a LoadBalancer instance for the database instance, which will incur costs from cloud vendor.
```
kbcli cluster expose ${cluster-name} --type internet --enable=true
```
To disable the LoadBalancer instance, execute the following command.
```
kbcli cluster expose ${cluster-name} --type internet --enable=false
```
>Note: the instance is inaccessible after you disable the LoadBalancer instance.
##Procedure 5. For temporary use, the client can connect to the database using an IP address if they are outside the Kubernetes cluster but in the same VPC
A domain name is not required. The KubeBlocks floating IP can be utilized for accessing the database. 
Enabling the kubeblocks load balancer feature is necessary to use this functionality. However, the availability of this feature differs across various cloud providers, as detailed below:
｜ Cloud Vendor  | Kuebrnetes ｜ Available ｜Remarks ｜
｜------------- | ------------- ｜ ------------- ｜ ------------- ｜
AWS  | EKS ｜ ✅ ｜- The loadbalancer will request ENI (Elastic Network Interface) and PrivateIP resources. The number of ENIs that a single machine can support and the number of PrivateIPs that an ENI can mount are dependent on the VM size, with larger VM sizes having larger quotas. Refer to：https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#AvailableIpPerENI
- The load balancer will request a Route 53 DNS domain name for the database cluster, which incurs certain costs. Refer to: https://aws.amazon.com/route53/pricing/
- The loadbalancer ensures that the instance domain name is stable, but when the instance switches across availability zones, the IP bound to the domain name may change. Please check your client and set a reasonable DNS cache time based on business tolerance.｜
Azure  | AKS ｜ ❌｜
GCP | GKE ｜ ❌｜


**Steps：**
1. Use```--set loadbalancer.enabled=true``` to enable load balancer when installing KubeBlocks.
```
kbcli kubeblocks install --set loadbalancer.enabled=true
```
If KubeBlocks is already enabled, you can upgrade it to enable load balancer.
```
kbcli kubeblocks upgrade --set loadbalancer.enabled=true
```
2. Use the following command to allow access to the cluster within the VPC.

```
kbcli cluster expose ${cluster-name} --type=kb-floating-ip --enable=true
```
3. Access the database in the VPC with external address. 
```
kbcli cluster describe x
Name: x         Created Time: Mar 01,2023 11:45 UTC+0800
NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
default     apecloud-mysql       ac-mysql-8.0.30   Running   Delete

Endpoints:
COMPONENT   MODE        INTERNAL                                 EXTERNAL
x           ReadWrite   x-mysql.default.svc.cluster.local:3306   10.0.2.203:3306

Topology:
COMPONENT   INSTANCE    ROLE     STATUS    AZ                NODE                                                       CREATED-TIME
mysql       x-mysql-0   leader   Running   cn-northwest-1b   ip-10-0-2-184.cn-northwest-1.compute.internal/10.0.2.184   Mar 01,2023 11:45 UTC+0800

Resources Allocation:
COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
mysql       false       1 / 1                1Gi / 1Gi               data:10Gi      <none>

Images:
COMPONENT   TYPE    IMAGE
mysql       mysql   registry.cn-hangzhou.aliyuncs.com/apecloud/apecloud-mysql-server:8.0.30-5.alpha2.20230105.gd6b8719.2

Events(last 5 warnings, see more:kbcli cluster list-events -n default x):
TIME   TYPE   REASON   OBJECT   MESSAGE
```

To stop allowing access, execute the following command. 
```
kbcli cluster expose ${cluster-name} --type=kb-floating-ip --enable=false
```

## Procedure 6. The client is outside the Kubernetes cluster but in the same VPC as the Kubernetes cluster
A stable domain name for long-term connections is required.An Internal LoadBalancer provided by the cloud vendor can be used for this purpose.
>Note: The following command will create a LoadBalancer instance for the database instance, which will incur costs from cloud vendor.
```
kbcli cluster expose ${cluster-name} --type vpc --enable=true
```
To disable the LoadBalancer instance, execute the following command.
>Note: Once disabled, the instance is not accessible.

```
kbcli cluster expose ${cluster-name} --type vpc --enable=false
```