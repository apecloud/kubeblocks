---
title: ChatGPT Retrieval Plugin with KubeBlocks
description: Introduction and Installation of ChatGPT Retrieval Plugin on KubeBlocks
keywords: [ChatGPT retrieval plugin]
sidebar_position: 6
sidebar_label: ChatGPT Retrieval Plugin
---

# ChatGPT Retrieval Plugin with KubeBlocks

## What is ChatGPT Retrieval Plugin

[ChatGPT Retrieval Plugin](https://github.com/openai/chatgpt-retrieval-plugin) is the official plugin from OpenAI, it provides a flexible solution for semantic search and retrieval of personal or organizational documents using natural language queries.

The purpose of Plugin is to extend the OpenAI abilities from a public large model to a hybrid model with private data. It manages a private knowledge base to promise the privacy and safety, meanwhile, it can be accessed through a set of query APIs from the remote OpenAI GPT Chatbox.

The remote Chatbox engages in dialogue with the user, processing their natural language queries and retrieving relevant data from both OpenAI's large model and our private knowledge base, and then combines this information to provide a more accurate and comprehensive answer.

With the ChatGPT Retrieval Plugin, data privacy and benefits of ChatGPT are balanced.

## ChatGPT Retrieval Plugin on KubeBlocks

KubeBlocks makes two major improvements over ChatGPT Retrieval Plugin:

- KubeBlocks provides a solid vector database for Plugin, and relieves the burden of database management. KubeBlocks supports a wide range of vector databases, such as Postgres, Redis, Milvus, Qdrant and Weaviate.
- KubeBlocks also builds multi-arch images for Plugin, integrates the Plugin as a native Cluster inside, you can configure the APIs, Secrets, Env vars configurable through command line and Helm Charts.

These improvements achieve a better experience for running your own Plugin.

## Install ChatGPT Retrieval Plugin on KubeBlocks

***Before you start***

- Make sure you have the Plugin authorities from OpenAI. If don't, you can join the [waiting list here](https://openai.com/waitlist/plugins).
- An OPENAI_API_KEY for the Plugin to call OpenAI embedding APIs.
- [Install kbcli and KubeBlocks](./../installation/install-and-uninstall-kbcli-and-kubeblocks.md).

***Steps:***

1. List addons of KubeBlocks. Each vector database is an addon in KubeBlocks.

   ```bash
   kbcli addon list 
   ```

   This guide shows how to install ChatGPT Retrieval Plugin with Qdrant. If Qdrant is not enabled, enable it with the following command.

   ```bash
   kbcli addon enable qdrant 
   ```

2. When enabled successfully, you can check it with `kbcli addon list` and `kubectl get clusterdefinition`.

   ```bash
   kbcli addon list 
   >
   NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   AUTO-INSTALLABLE-SELECTOR
   qdrant                         Helm   Enabled                   true
   ```

   ```bash
   kubectl get clusterdefinition
   >
   NAME   MAIN-COMPONENT-NAME STATUS    AGE
   qdrant qdrant              Available 6m14s
   ```

3. Create a Qdrant cluster with `kbcli cluster create --cluster-definition=qdrant`.

   ```bash
   kbcli cluster create --cluster-definition=qdrant
   >
   Warning: cluster version is not specified, use the recently created ClusterVersion qdrant-1.1.0
   Cluster lilac26 created
   ```

   ***Result:***

   In the above example, a qdrant standalone cluster named `Cluster lilac26` is created.

4. Install the plugin with Qdrant as datastore with helm.

   ```bash
   helm install gptplugin kubeblocks/chatgpt-retrieval-plugin \
   --set datastore.DATASTORE=qdrant \
   --set datastore.QDRANT_COLLECTION=document_chunks \
   --set datastore.QDRANT_URL=http://lilac26-qdrant-headless.default.svc.cluster.local \
   --set datastore.QDRANT_PORT=6333 \
   --set datastore.BEARER_TOKEN=your_bearer_token \
   --set datastore.OPENAI_API_KEY=your_openai_api_key \
   --set website.url=your_website_url
   ```

5. Check whether the plugin is installed successfully.

   ```bash
   kubectl get pods
   >
   NAME                                                READY STATUS  RESTARTS AGE
   gptplugin-chatgpt-retrieval-plugin-647d85498d-jd2bj 1/1   Running 0        10m
   ```

6. Port-forward the Plugin Portal to access it.

   ```bash
   kubectl port-forward pod/gptplugin-chatgpt-retrieval-plugin-647d85498d-jd2bj 8081:8080
   ```

7. In your web browser, open the plugin portal with the address `http://127.0.0.1:8081/docs`.
