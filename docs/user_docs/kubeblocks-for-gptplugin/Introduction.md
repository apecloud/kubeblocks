## Introduction to KubeBlocks for ChatGPT Retrieval Plugin
### ChatGPT Retrieval Plugin
[ChatGPT Retrieval Plugin](https://github.com/openai/chatgpt-retrieval-plugin) is the official plugin from OpenAI, it provides a flexible solution for semantic search and retrieval of personal or organizational documents using natural language queries.   
The Plugin manages a private documents store to promise the privacy and safety, meanwhile, it can be accessed through a set of query APIs from the remote OpenAI GPT Chatbox.   
The remote Chatbox is responsible for the dialogue with user, it processes the natural language query, retrieves data from both of OpenAI large model and private store, combines results to provide a better answer.  
With the boost of private and domain data, the results could be more reasonable, accurate and personal.
It is also a good way to balance the data privacy and utilization.

### Requirements from OpenAI
Two prerequisites are needed for running the Plugin:  
The Plugin authorities from OpenAI, if none, you can join the [waiting list here](https://openai.com/waitlist/plugins).  
An OPENAI_API_KEY for the Plugin to call OpenAI embedding APIs. 

### Plugin on KubeBlocks
KubeBlocks makes two major improvements over ChatGPT Retrieval Plugin:  
KubeBlocks provides a solid vector database for Plugin, and relieves the burden of database management. Right now, KubeBlocks support a wide range of vector databases, such as Postgres, Redis, Milvus, Qdrant and Weaviate. The support of other vector databases is on the way.  
KubeBlocks also builds multi-arch images for Plugin, integrates the Plugin as a native Cluster inside, makes the APIs, Secrets, Env vars configurable through command line & Helm Charts.  
These improvements achieve a better experience for building your own Plugin.

### Requirements from KubeBlocks
Kbcli & the lastest KubeBlocks edition are installed.
TODO: the installation part of kbcli & kb.

### Installation
#### Step 1: Create a vector database with kbcli 
1. List Addons of KubeBlocks, each vector database is an addon in KubeBlocks
```shell
kbcli addon list 
```
2. If not enabled, enable it with
```shell
kbcli addon enable qdrant 
```
waiting for the addon from 'enabling' to 'enabled'
3. When enabled successfully, you can check it with
```shell
kbcli addon list 
kubectl get clusterdefintion

NAME                MAIN-COMPONENT-NAME   STATUS      AGE
qdrant-standalone   qdrant                Available   6m14s
```
4. create a qdrant cluster
```shell
kbcli cluster create  --cluster-definition=qdrant-standalone

Warning: cluster version is not specified, use the recently created ClusterVersion qdrant-1.1.0
Cluster lilac26 created
```
a qdrant standalone cluster is created successfully
#### Step 2: Start the plugin with qdrant as store
```shell
kbcli addon enable chatgpt-retrieval-plugin 
--set datastore.DATASTORE=qdrant 
--set datastore.QDRANT_COLLECTION=document_chunks
--set datastore.QDRANT_URL=http://lilac26-qdrant-headless.default.svc.cluster.local 
--set datastore.QDRANT_PORT=6333 
--set datastore.BEARER_TOKEN=your_bearer_token
--set datastore.OPENAI_API_KEY=your_openai_api_key 
--set website.url=your_website_url

kubectl get pods -n kb-system
NAME                                                     READY   STATUS      RESTARTS   AGE
kb-addon-chatgpt-retrieval-plugin-77f46b7b7c-j92g2       0/1     Running     0          7s
```
or with helm to install 
```shell
helm install gptplugin 
--set datastore.DATASTORE=qdrant 
--set datastore.QDRANT_COLLECTION=document_chunks
--set datastore.QDRANT_URL=http://lilac26-qdrant-headless.default.svc.cluster.local 
--set datastore.QDRANT_PORT=6333 
--set datastore.BEARER_TOKEN=your_bearer_token
--set datastore.OPENAI_API_KEY=your_openai_api_key 
--set website.url=your_website_url

kubectl get pods
NAME                                                  READY   STATUS    RESTARTS     AGE
gptplugin-chatgpt-retrieval-plugin-647d85498d-jd2bj   1/1     Running   0            10m
```

#### Step 3: Port-forward the Plugin Portal
```shell
kubectl port-forward pod/gptplugin-chatgpt-retrieval-plugin-647d85498d-jd2bj 8081:8080
```
In your web browser, open the plugin portal with
```shell
http://127.0.0.1:8081/docs
```
