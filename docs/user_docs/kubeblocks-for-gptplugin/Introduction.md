## Introduction to KubeBlocks for ChatGPT Retrieval Plugin
### ChatGPT Retrieval Plugin
[ChatGPT Retrieval Plugin](https://github.com/openai/chatgpt-retrieval-plugin) is the official plugin from OpenAI, it provides a flexible solution for semantic search and retrieval of personal or organizational documents using natural language queries.  
The purpose of Plugin is to extend the OpenAI abilities from a public large model to a hybrid model with private data. It manages a private documents store to promise the privacy and safety, meanwhile, it can be accessed through a set of query APIs from the remote OpenAI GPT Chatbox.   
The remote Chatbox is responsible for the dialogue with user, it processes the natural language query, retrieves data from both of OpenAI large model and private store, combines results to provide a better answer.  
With the boost of private and domain data, the results could be more reasonable, accurate and personal.
It is a good way to balance the data privacy and utilization.

### Plugin on KubeBlocks
KubeBlocks makes two major improvements over ChatGPT Retrieval Plugin:  
KubeBlocks provides a solid vector database for Plugin, and relieves the burden of database management. Right now, KubeBlocks support a wide range of vector databases, such as Postgres, Redis, Milvus, Qdrant and Weaviate. The support of other vector databases is on the way.  
KubeBlocks also builds multi-arch images for Plugin, integrates the Plugin as a native Cluster inside, makes the APIs, Secrets, Env vars configurable through command line & Helm Charts.  
These improvements achieve a better experience for building your own Plugin.
