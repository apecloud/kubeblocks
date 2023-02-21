---
title: Connect database from anywhere
description: How to connect to a database
sidebar_position: 1
---

# Connect database from anywhere 
The `kbcli cluster expose` command is used to expose the database cluster service address to the outside, so that you can access the cluster from non-kubernetes nodes in the same VPC(Virtual Private Cloud).

## Before you start
Make sure that loadbalancer service is enabled. If the load balancer is not deployed, `expose` command does not take effect but triggers no errors.

## Expose floating IP address
Execute the following command:
```
➜ KBCLI_EXPERIMENTAL_EXPOSE="1" kbcli cluster expose --help
Expose a database cluster

Options:
    --off=false:
        Stop expose a database cluster

    --on=false:
        Expose a database cluster

Usage:
  kbcli cluster expose [flags] [options]

Use "kbcli options" for a list of global command-line options (applies to all commands).
➜
```
You can use the `kbcli cluster describe` command to query the Component Floating IP. The example is as follows. In the following example, the external address is the floating IP address. If the component does not expose services to the outside of the cluster, the External field does not display.

*Example*
TODO This example needs to be rewritten

```
Block:
  Type:      consensus
  Replicas:  3 desired | 3 total
  Status:    3 Running / 0 Waiting / 0 Succeeded / 0 Failed  
  Image:   docker.io/apecloud/apecloud-mysql-server:latest
  Cpu:          1000m
  Memory:       1024Mi
  Endpoints:
    ReadWrite:
      Internal: 10.42.0.13:3306/TCP
      External: 10.42.0.13:3306/TCP
    ReadOnly:
      Internal: 10.42.0.13:3306/TCP 
      External: 10.42.0.13:3306/TCP  
```
