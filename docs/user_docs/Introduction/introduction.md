# What is KubeBlocks
## Backgrounds
KubeBlocks, running on K8S, offers a universal view through multi-clouds and on-premises, provides a consistent experience for fully managing multiple databases, and relieves the burden of maintaining miscellaneous operators. Data-driven organizations run multiple databases in many platforms, they have a typical data model which uses NoSQL(Redis) for caching, SQL(MySQL/PostgreSQL) for transaction processing and a Big Data suite(Kafka/Flink/Hadoop) for data mining and monetization. Meanwhile, new databases emerge in endlessly in various business domain, such as the GraphDB for social network and TimeSeriesDB for Internet of things. However, running multiple databases is a great challenge for DevOps team, especially when the deployment is cross multi-clouds or even in hybrid mode with legacy systems on-premises. With the rise of Kubernetes, many organizations move their workloads including databases to it, and this proposes another challenge for developers to maintain multiple database operators from multiple sources. Kubeblocks is designed for multi-clouds and on-premise system as well. 


## Design Principles
### Statefulset Oriented 
More precisely, KubeBlocks is database oriented. Database is always composed of a group of dependent StatefulSets, the most complicated part is the inner replication and synchronization relations, we drill down dozens of popular databases and summarize several typical replication sets such as ConsensusSet and ReplicationSet based on the replication relations. A replication set is the minimum set of a replication group on top of a group of Kubernetes StatefulSets，with a replication strategy to relate them together. With such abstraction, it breaks down a database which is also a complicated distributed system into loose-coupled stateless, stateful or replication sets. Each replication set looks like a block of a marvel building.

### Database Engine Irrelevant  
Like the StatefulSet in Kubernetes is irrelevant to applications, replication set in KubeBlocks is irrelevant to specific database engines. KubeBlocks focuses on the topology of a database and the mapping of each block to a corresponding set. With such design, any database or stateful system can be plugged in seamlessly with certain configurable topology definitions.

### Domain Driven 
Specificly designed for database, KubeBlocks also aims to offer a holistic solution for database management. It employs the DDD philosophy for the domain problems. There are many inevitable platform functions like backup, monitoring, account, high availability, etc. These domain functions are also designed to be irrelevant to specific database engine.

### Operatorless 
Helped with the irrelevance design and equipped with abundant domain functions, a database can be integrated into KubeBlocks efficiently，then acquires the platform abilities immediately without a customized operator. In general, KubeBlocks shows an operatorless design for databases.

### Declarative API 
Declare the desired abilities a database needs in YAML configurations, apply it with KubeBlocks, then a state-of-art database management platform is on hand. A database provider has no need to care about workflows and struggle with the infrastructure. 
### Cloud-Native & Cloud-Neutral 
KubeBlocks is designed for multi-clouds, it shields discrepancies in clouds, enables users to conduct their multi-clouds strategy. It is also open-sourced to relieve the anxiety about vendor lock-in.