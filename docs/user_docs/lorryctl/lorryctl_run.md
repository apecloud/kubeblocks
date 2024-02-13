---
title: lorryctl run
---

Run Lorry and db service.

```
lorryctl run [flags]
```

### Examples

```

lorryctl run  -- mysqld
  
```

### Options

```
  -d, --components-path string        The path for components directory (default "/kubeblocks/config/probe/components")
  -c, --config string                 Dapr configuration file (default "/kubeblocks/config/probe/config.yaml")
  -G, --dapr-grpc-port int            The gRPC port for Dapr to listen on (default -1)
  -H, --dapr-http-port int            The HTTP port for Dapr to listen on (default -1)
  -I, --dapr-internal-grpc-port int   The gRPC port for the Dapr internal API to listen on (default 56471)
  -h, --help                          Print this help message
      --log-level string              The log verbosity. Valid values are: debug, info, warn, error, fatal, or panic (default "info")
```

### Options inherited from parent commands

```
      --kb-runtime-dir string   The directory of kubeblocks binaries (default "/kubeblocks/")
```

### SEE ALSO

* [lorryctl](lorryctl.md)	 - LORRY CLI

#### Go Back to [LorryCtl Overview](cli.md) Homepage.

