 # Connect database from anywhere 
The `dbctl cluster expose` command is used to expose the database cluster service address to the outside, so that you can access the cluster from non-kubernetes nodes in the same VPC(Virtual Private Cloud).

## Before you start
Make sure that loadbalancer service is enabled. If the load balancer is not deployed, `expose` command does not take effect but triggers no errors.

## Expose floating IP address
Execute the following command:
```
~/git/kubeblocks
➜ DBCTL_EXPERIMENTAL_EXPOSE="1" bin/dbctl cluster expose --help
Expose a database cluster

Options:
    --off=false:
        Stop expose a database cluster

    --on=false:
        Expose a database cluster

Usage:
  dbctl cluster expose [flags] [options]

Use "dbctl options" for a list of global command-line options (applies to all commands).
~/git/kubeblocks
➜
```
You can use the `dbctl cluster describe` command to query the Component Floating IP. The example is as follows. In the following example, the external address is the floating IP address. If the component does not expose services to the outside of the cluster, the External field does not display.
*Example*
```
Block: ## TODO this an array/map
  Type:      consensus
  Replicas:  3 desired | 3 total
  Status:    3 Running / 0 Waiting / 0 Succeeded / 0 Failed  
  Image:   docker.io/infracreate/wesql-server-8.0.29:0.1-SNAPSHOT
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

code example需要确认，内容还需要补充。