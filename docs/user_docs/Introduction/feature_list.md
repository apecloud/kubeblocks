# Feature list

- Cloud-neutral
  - Runs on a Kubernetes base and supports AWS EKS, GCP GKE, Azure AKS and other cloud environments
  
- Database as code
  - Defines each supported database engine through a declarative API, extending the Kubernetes statefulset to better adapt to stateful services as databases
  - Developers can use Kubernetes CLI or API to interact with KubeBlocks database clusters, integrate into DevOps tools and processes

- Multiple database engines compatible
  - Compatible with MySQL, PostgreSQL, Redis, MongoDB and other database engines
  - You can also access and manage any new database engines or Plugins by defining CRD (Kubernetes custom resource definition)
  
- High availability
  - Provides a high-availability database WeSQL that is fully compatible with MySQL, and supports single-availability zone deployment and three-availability zone deployment
  - Based on the consistency X-Paxos protocol, WeSQL cluster realizes automatic master selection, log synchronization, and strong data consistency, and the cluster maintains high availability when a single availability zone failure
  - Follows MySQL Binary Log standard, compatible with commonly used Binlog incremental subscription tools
  
- Life cycle management
  - Pause and resume the database cluster
  - Restart the cluster
  - Vertical scaling, change the CPU and memory configuration of cluster nodes
  - Horizontal scaling to increase read replicas
  - Volume expansion
  - Database parameter modification
  
- Backup and restore
  - File backup and snapshot backup, realize minute-level snapshot backup through EBS
  - User-defined backup tools
  - Automatic backup and manual backup
  - Backup files are stored in object storage such as S3, GCS, etc., and you can specify the retention period and number of retention
  - Restores to new database cluster or restore to the original cluster
  
- Monitoring and alarming
  - Built-in Prometheus, Grafana and AlertManager
  - Supports Prometheus exporter to output Metrics
  - Customized Grafana dashboard to view and monitor the dashboard
  - Support AlertManager to define alert rules and notifications
  
- Cost-effective
  - KubeBlocks is completely free and open-source
  - Support resource overcommitment
  
- Safety
  - Role-based access control (RBAC)
  - Network transmission encryption
  - Data storage encryption
  - Backup encryption
  
- kbcli - Easy-to-use CLI command line tool
  - Install, uninstall and upgrade the system with kbcli
  - Support common operations such as database cluster, backup and recovery, monitoring, log, operation and maintenance, bench
  - Support kbcli to connect to the database cluster without repeatedly entering a password
  - Support command line automatic completion