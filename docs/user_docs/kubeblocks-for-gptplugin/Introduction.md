## Introduction to KubeBlocks for ChatGPT Retrieval Plugin
### ChatGPT Retrieval Plugin
[ChatGPT Retrieval Plugin](https://github.com/openai/chatgpt-retrieval-plugin) is the official plugin from OpenAI, it provides a flexible solution for semantic search and retrieval of personal or organizational documents using natural language queries.   
The Plugin manages a private documents store to promise the privacy and safety, meanwhile, it can be accessed through a set of query APIs from the remote OpenAI GPT Chatbox.   
The remote Chatbox is responsible for the dialogue with user, it processes the natural language query, retrieves data from both of OpenAI large model and private store, combines results to provide a better answer.  
With the boost of private and domain data, the results could be more reasonable, accurate and personal.
It is also a good way to balance the data privacy and utilization.

### Requirements from OpenAI
The Plugin authorities from OpenAI, if none, you can join the [waiting list here](https://openai.com/waitlist/plugins).  
An OPENAI_API_KEY for the Plugin to call OpenAI embedding APIs.

### Requirements from KubeBlocks
Kbcli & the lastest KubeBlocks edition are installed.

### Installation
Install the private document store.  
Install the ChatGPT Retrieval Plugin.  

