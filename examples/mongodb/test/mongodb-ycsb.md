## YCSB Test

We will use the open-source YCSB 0.17.0 benchmarking tool for performance testing.

### **Load test data using YCSB tool**

```bash load-command.sh
./bin/ycsb.sh load mongodb -s -p workload=site.ycsb.workloads.CoreWorkload -p recordcount=10000 -p mongodb.url=mongodb://<user>:<password>@<host>:<port>/admin?w=0 -threads 4
```

You need to modify the following parameters:

- `recordcount`: Total number of records to load into MongoDB instance
- `mongodb.url=mongodb://root:REDACTED@host:27017/admin`: Connection URL for MongoDB instance. This example uses admin database.
- `threads`: Client concurrency thread count

### **Execute performance test**

```bash run-command.sh
./bin/ycsb.sh run mongodb -s -p workload=site.ycsb.workloads.CoreWorkload -p recordcount=10000 -p operationcount=50000 -p insertproportion=0 -p readproportion=50 -p updateproportion=50 -p requestdistribution=zipfian -p mongodb://<user>:<password>@<host>:<port>/admin?w=0 -threads 4
```

Parameters to modify:

- `recordcount`: Total loaded data volume
- `operationcount`: Total read/write operations
- `insertproportion`: Data loading operation ratio (0 means no inserts during test)
- `readproportion`: Read operation percentage
- `updateproportion`: Update operation percentage
- `mongodb.url=mongodb://<user>:<password>@<host>:<port>/admin`: MongoDB connection URL.  This example uses admin database.
- `threads`: Client concurrency thread count

### Run YCSB using Docker Image

We packed a docker image from YCSB 0.17.0 for test. Dockerfile is available at [YCSB Dockerfile](./Dockerfile-ycsb)

You can run TEST using (or you can build image from the Dockerfile provided)

```bash
docker run docker.io/apecloud/ycsb:0.17.0 /bin/sh -c './bin/ycsb.sh load mongodb -s -p workload=site.ycsb.workloads.CoreWorkload -p recordcount=10000 -p mongodb.url=mongodb://<user>:<password>@<host>:<port>/admin?w=0 -threads 4'
docker run docker.io/apecloud/ycsb:0.17.0 /bin/sh -c './bin/ycsb.sh run mongodb -s -p workload=site.ycsb.workloads.CoreWorkload -p recordcount=10000 -p operationcount=50000 -p insertproportion=0 -p readproportion=50 -p updateproportion=50 -p requestdistribution=zipfian -p mongodb.url=mongodb://<user>:<password>@<host>:<port>/admin?w=0 -threads 4'
```

Besides, PingCAP provides go-ycsb: <https://github.com/pingcap/go-ycsb>. If you are a go-lover, you may run ycsb bench using go-ycsb.
