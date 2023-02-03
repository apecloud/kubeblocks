# Feature list

* Kubernetes native and multicloud supported
  Based on Kubernetes and cloud-neutral
  * KubeBlocks greatly simplifies the process of deploying a database on Kubernetes. You can deploy and use a database cluster within several minutes without the knowledge of Kubernetes.
  * Runs on a Kubernetes base and supports AWS EKS, GCP GKE, Azure AKS, and other cloud environments.
  * Defines each supported database engine through a declarative API, extending the Kubernetes statefulset to better adapt to stateful services as databases.
  * Developers can use Kubernetes CLI or API to interact with KubeBlocks database clusters and integrate them into DevOps tools and processes to realize Database Infrastructure as Code.

* Multiple database engines compatible
  * Built-in MySQL, PostgreSQL, Redis, and other database engines.
  * You can also access and manage any new database engines or Plugins by defining CRD (Kubernetes custom resource definition).

* High availability
  * Provides a high-availability database cluster, ApeCloud MySQL cluster, which is fully compatible with MySQL, and supports single-availability zone deployment, double-availability zone deployment, and three-availability zone deployment.
  * Based on the Paxos consensus protocol, the ApeCloud MySQL cluster realizes automatic leader election, log synchronization, and strict consistency, and the cluster maintains high availability when one availability zone fails.
  * Follows MySQL Binary Log standard and maintains compatibility with commonly used Binlog incremental subscription tools.

* Life cycle management
  * Supports pausing and resuming database clusters.
  * Supports restarting clusters.
  * Supports vertical scaling to change the CPU and memory configuration of cluster pods.
  * Supports horizontal scaling to increase read replicas.
  * Supports volume expansion.
  * Supports database parameter modification.

* Backup and restore
  * Supports file backup and snapshot backup and realizes minute-level snapshot backup through EBS.
  * Supports user-defined backup tools.
  * Supports automatic backup and manual backup.
  * Supports storing backup files in object storage such as S3, and you can specify the retention amount.
  * Restores the original database cluster through backup.

* Monitoring and alarm
  * Built-in Prometheus, Grafana, and AlertManager.
  * Supports Prometheus exporter to output Metrics.
  * Customized Grafana dashboard to view and monitor the dashboard.
  * Supports AlertManager to define alert rules and notifications.

* Cost-effective
  * KubeBlocks is completely free and open-source.
  * You can choose your own lower-cost VM instances, such as reserved instances with one- to three-year terms.
  * Supports resource overcommitment to allocate more database instances on one EC2 to decrease costs efficiently.

* Safety
  * Role-based access control (RBAC)
  * Supports network transmission encryption.
  * Supports data storage encryption.
  * Supports backup encryption.

* `kbcli`, an easy-to-use CLI
  * Supports installing, uninstalling, and upgrading the system with kbcli.
  * Supports common operations such as database cluster, backup and recovery, monitoring, log, operation, and maintenance.
  * Supports using `kbcli` to connect to the database cluster without repeatedly entering a password.
  * Supports command line automatic completion.