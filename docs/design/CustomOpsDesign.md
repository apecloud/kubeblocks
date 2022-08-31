**Custom OpsDefinition概要设计**

# 变更记录
|  版本   | 时间         | 变更记录  |
|  ----  |------------| ----|
| 1.0.0  | 2022.8.24   | 初始版本|

# 提案评审信息
|  评审人   | 必选/可选 评审者 | 是否通过？| 评审日期
|  ----  | ----  | ----| ----|
| dullboy  | 必选 | [x] 通过</br>[x] 评审意见待解决 | |
| 聪心  | 必选 | [x] 通过</br>[] 评审意见待解决 | |
| 燧木  | 必选 | [x] 通过</br>[x] 评审意见待解决 | |

# DBaaSv0-Custom OpsDefinition 概要设计

## 提案概览

该提案支持面向ISV的自定义运维操作的能力

## 术语及概念

（在这里填写专用术语及概念）

## 设计目标与动机

Dbass平台会支持生命周期的通用运维操作，但是偏向于业务场景的任务，不会抽象定义。这些场景包括：某个数据库不可用了，但是这个失败修复过程ISV是已知的，已经有对应处理脚本，就能通过这个Custom
OpsDefinition接入平台；还有kafka独有的修改Topic分区或者配置的操作等等。

我们需要提供一个扩展入口，支持将这些业务场景的任务接入平台，方便用户灵活扩展和使用，同时可以使用平台提供的相关能力（包括可以指定到对应引擎pod执行、获得引擎相关连接和用户信息等等）。

## 目标需求

 •
 对于偏向业务的运维场景，提供声明式的扩展入口允许ISV接入，暂时不提供任何运行约束

## 提案背景与限制说明

### 提案背景
>
> Dbass是一个多引擎数据库管理平台，我们通过抽象和定义通用运维操作的声明式API，快速接入运维操作到平台。但是很多数据库引擎存在偏业务场景的运维任务，比如
> kafka的修改topic的配置和分区（其他数据库没有）等等；我们就需要考虑这些特定任务如何方便的接入平台。
>
### 替代方案

### 用户故事

 （如果没有具体用户场景*/*故事，可以写设计现状、背景等）

 下面的都是引擎特殊的运维场景。

#### 场景1

 influxdb的rebalance，每个步骤都是一些命令，然后得根据命令输出进行分析结果是否正确。

> 第 1 步：截断热分片
>
> 防止数据不一致，会创建一个新分片他数据都写入这个分片，以前的变冷分片，后面只要重新平衡冷分片就行。
>
> Plaintext  
> influxd-ctl truncate-shards  
> 输出：Truncated shards.
>
> 第 2 步：识别冷分片
>
> 第 3 步：复制冷分片
>
> 第 4 步：确认复制的分片
>
> 第 5 步：移除不必要的冷碎片
>
> 第 6 步：确认重新平衡
>
#### 场景2

 kafka的opertions有下面几种(很多都是偏向业务的运维任务)：

 • 增加和删除topic

```
 bin/kafka-topics.sh --bootstrap-server broker_host:port --create
 --topic my_topic_name \\  
 --partitions 20 --replication-factor 3 --config x=y
```
 •
 rebalance集群，分区重分配，示例：主题foo1,foo2的分区分配到新节点5,6上

```  
cat topics-to-move.json  
 {"topics": \[{"topic": "foo1"},  
 {"topic": "foo2"}\],  
 "version":1  
 }
```
 生成分配plan

 ```
 bin/kafka-reassign-partitions.sh --bootstrap-server localhost:9092
 --topics-to-move-json-file topics-to-move.json --broker-list "5,6"
 --generate  
 Current partition replica assignment

 {"version":1,  
 "partitions":\[{"topic":"foo1","partition":0,"replicas":\[2,1\]},  
 {"topic":"foo1","partition":1,"replicas":\[1,3\]},  
 {"topic":"foo1","partition":2,"replicas":\[3,4\]},  
 {"topic":"foo2","partition":0,"replicas":\[4,2\]},  
 {"topic":"foo2","partition":1,"replicas":\[2,1\]},  
 {"topic":"foo2","partition":2,"replicas":\[1,3\]}\]}

 Proposed partition reassignment configuration

 {"version":1,  
 "partitions":\[{"topic":"foo1","partition":0,"replicas":\[6,5\]},  
 {"topic":"foo1","partition":1,"replicas":\[5,6\]},  
 {"topic":"foo1","partition":2,"replicas":\[6,5\]},  
 {"topic":"foo2","partition":0,"replicas":\[5,6\]},  
 {"topic":"foo2","partition":1,"replicas":\[6,5\]},  
 {"topic":"foo2","partition":2,"replicas":\[5,6\]}\]}
```
 执行plan

```
bin/kafka-reassign-partitions.sh --bootstrap-server localhost:9092
 --topics-to-move-json-file topics-to-move.json --broker-list "5,6"
 --generate  
 Current partition replica assignment

 {"version":1,  
 "partitions":\[{"topic":"foo1","partition":0,"replicas":\[2,1\]},  
 {"topic":"foo1","partition":1,"replicas":\[1,3\]},  
 {"topic":"foo1","partition":2,"replicas":\[3,4\]},  
 {"topic":"foo2","partition":0,"replicas":\[4,2\]},  
 {"topic":"foo2","partition":1,"replicas":\[2,1\]},  
 {"topic":"foo2","partition":2,"replicas":\[1,3\]}\]}

 Proposed partition reassignment configuration

 {"version":1,  
 "partitions":\[{"topic":"foo1","partition":0,"replicas":\[6,5\]},  
 {"topic":"foo1","partition":1,"replicas":\[5,6\]},  
 {"topic":"foo1","partition":2,"replicas":\[6,5\]},  
 {"topic":"foo2","partition":0,"replicas":\[5,6\]},  
 {"topic":"foo2","partition":1,"replicas":\[6,5\]},  
 {"topic":"foo2","partition":2,"replicas":\[5,6\]}\]}
```
 • 修改topic的配置和分区
```
修改分区  
 bin/kafka-topics.sh --bootstrap-server broker_host:port --alter
--topic my_topic_name \\  
--partitions 40  
修改配置  
 bin/kafka-configs.sh --bootstrap-server broker_host:port
--entity-type topics --entity-name my_topic_name --alter --add-config
x=y
```
####场景3

redis的重新平衡分片。

``` 
redis-cli --cluster reshard \<host\>:\<port\> --cluster-from \<node-id\>
--cluster-to \<node-id\> --cluster-slots \<number of slots\>
--cluster-yes
```
####场景4

ElasticSearch

 • 重置密码
```
 bin/elasticsearch-reset-password
 [-a, --auto] [-b, --batch] [-E <KeyValuePair]
 [-f, --force] [-h, --help] [-i, --interactive]
 [-s, --silent] [-u, --username] [--url] [-v, --verbose]
```
 • 分片相关操作
```
bin/elasticsearch-shard remove-corrupted-data
  ([--index <Index>] [--shard-id <ShardId>] | [--dir <IndexPath>])
  [--truncate-clean-translog]
  [-E <KeyValuePair>]
  [-h, --help] ([-s, --silent] | [-v, --verbose])
```
还有很多其他命令行工具，参考https://www.elastic.co/guide/en/elasticsearch/reference/8.3/commands.html。

##架构设计（模块、架构图）

*(*注意：架构图建议原创，后续会给出图片的规则。*)*

###架构设计图

###外部接口设计

####OpsDefinition

调研了5种数据库
kafka,redis,mysql,influxdb,elasticSearch，发现特定或者偏业务类型的运维操作，都是一个命令或者多个命令组成（也就是自动化脚本）。

所以我们应该要提供一个自定义任务的抽象，支持参数传入、运行脚本和命令、指定在哪个组件和角色中执行命令；也可以使用用户提供的toolImage，创建k8s的job任务运行。
``` yaml
apiVersion: opendbaas.infracreate.com/v1alpha1
kind: OpsDefinition
metadata:
  name: redis-reshard
spec:
  # 关联的clusterDefinition
  clusterDefinitionRef: a-vendor-redis-clsuter
  type: Custom
  # 描述这个opsDefinition是干嘛的，可以在console呈现
  # 同时点击按钮的时候，可以生成执行计划
  description: "redis reshard"
  # 可以外部传参
  parameters:
    - name: cluster-slots
      value: 1024
    - name: port
      value: 7000
    - name: nodeId
      value: 07c37dfeb235213a872192d90877d0cd55635b91
  strategies:
    # 重试次数
    retryTimes: 3
    # 规范跟k8s容器一样，会起一个job运行
    container:
      # 如果image没有填写，到指定组件和roleGroup执行
      # name: engine 会到指定容器执行
      image: redis-cli:lastest
      command: [sh,-c]
      args: ["redis-cli --cluster reshard {{HOST}}:{{parameters.port}} --cluster-from {{parameters.nodeId}} --cluster-to {{parameters.nodeId}} --cluster-slots {{parameters.cluster-slots}} --cluster-yes"]
    components:
    -  type: master-slave
       roleGroups: [master]
       # tip: redis-cluster是多个主从节点群组成的分布式服务集群
       # 从这个组件的master角色中，挑选一个执行；
       pickOne: true 
```
####OpsRequest (User)

```yaml  
apiVersion: opendbaas.infracreate.com/v1alpha1
kind: OpsRequest
metadata:
  name: redis-reshard-job
spec:
  clusterRef: my-redis-clsuter
  opsDefinitionRef: redis-reshard
  parameters:
    - name: cluster-slots
      value: 1024
    - name: port
      value: 7000
    - name: nodeId
      value: 07c37dfeb235213a872192d90877d0cd55635b91
 
 status:
   # Cancelled  successfully  Failed Running
   phase: successfully
   tasks:
     redis-reshard:
       status: successfully
       finishedAt: "2022-07-29T00:00:00Z"
       startedAt: "2022-07-28T13:36:31Z"
       message: " redis-reshard successfully"
```
###内部组件设计
###开发计划
###兼容性（软硬件）
（如果与历史版本不兼容，需重点说明）
###技术指标
（例如规模、容量、性能等预设技术指标值）
###历史方案迁移（如有
（如果现有方案将替换历史方案，需考虑历史迁移）
###测试计划
####测试更新的前提条件
####单元测试方案
####集成测试方案
####端到端测试方案
####故障注入测试方案（如有）

#其他说明

##提案注意事项/限制条件/警告（如有

##风险与规避措施（如有）

##数据库安全（如有）

##部署历史（如有）
（基于已有的内容，做了哪些修改优化）
##需要的基础设施（如有）
##参考文档/文献（如有）