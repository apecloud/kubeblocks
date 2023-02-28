# What is KubeBlocks
## Backgrounds
Data-driven organizations run multiple databases in many platforms. Usually a successful online business has a data model with NoSQL(Redis) for caching, SQL(MySQL/PostgreSQL) for transaction processing and a Big Data suite(Kafka/Flink/Hadoop) for data mining and monetization. It adds the complexity with new databases emerging in a endlessly manner, such as GraphDB for social network and TimeSeriesDB for IoT. This poses a great challenge for DevOps team, especially when the deployment is on multicloud or even in hybrid mode with legacy systems on-premises. 

Cloud-native is on the rise and now Kubernetes is the environment provider for database,  organizations move their workloads including databases to it, so there is another challenge for DevOps team to maintain multiple database operators from multiple sources. 

These challenges motivate us to design a database management platform - KubeBlocks. Running on Kubernetes, KubeBlocks offers a universal view for multicloud and on-premises databases, provides a consistent experience, and relieves the burden of maintaining miscellaneous operators.

## Design Principles
### Statefulset Oriented : 
More precisely, KubeBlocks is database oriented. Database is always composed of a group of dependent StatefulSets, the most complicated part is the inner replication and synchronization relations, we drill down dozens of popular databases and summarize several typical replication sets such as ConsensusSet and ReplicationSet based on the replication relations. A replication set is a minimum set of a replication group which is a group of K8S StatefulSets，with a replication strategy to relate them together. With such abstraction, it seamlessly breaks down a database into loose coupled stateful replication sets. Each set looks like a block of a marvel building.
### Database Engine Irrelevant : 
Like the StatefulSet in K8S is irrelevant to applications, replication set in KubeBlocks is irrelevant to specific database engines. KubeBlocks focuses on the topology of a database and the mapping of each block to a corresponding set. With such design, any database or stateful system can be plugged in seamlessly with some configurable topology definitions.
### Domain Driven : 
As designed for database, KubeBlocks offers a total solution for database managements. It employs the DDD philosophy for solving the domain problems. There are many inevitable platform functions like backup, monitoring, high availability, etc. These domain functions are also designed to be irrelevant to specific database engine.
### Operatorless : 
Helped with the irrelevance design and equipped with abundant domain functions, a database can be integrated into KubeBlocks efficiently，then acquires the platform abilities immediately without a customized operator. In general, KubeBlocks shows an operatorless design for databases.
### Declarative API : 
Declare the desired abilities a database needs in YAML configurations, apply it with KubeBlocks, then a state-of-art database management platform is on hand. A database provider has no need to care about workflows and struggle with the infrastructure. 
### Cloud-Native & Cloud-Neutral : 
KubeBlocks is designed for multi-clouds, it shields discrepancies in clouds, enables users to conduct their multi-clouds strategy. It is also open-sourced to relieve the anxiety about vendor lock-in.
