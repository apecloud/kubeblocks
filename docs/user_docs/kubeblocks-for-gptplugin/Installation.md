## Installation of ChatGPT Retrieval Plugin on KubeBlocks

### Requirements from OpenAI
Two prerequisites are needed for running the Plugin:  
The Plugin authorities from OpenAI, if none, you can join the [waiting list here](https://openai.com/waitlist/plugins).  
An OPENAI_API_KEY for the Plugin to call OpenAI embedding APIs. 

### Requirements from KubeBlocks
Kbcli & the latest KubeBlocks edition are installed.
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
qdrant              qdrant                Available   6m14s
```
4. create a qdrant cluster
```shell
kbcli cluster create  --cluster-definition=qdrant

Warning: cluster version is not specified, use the recently created ClusterVersion qdrant-1.1.0
Cluster lilac26 created
```
a qdrant cluster is created successfully
#### Step 2: Start the plugin with qdrant as store
with helm to install 
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
