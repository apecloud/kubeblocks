---
title: lorryctl switchover
---

execute a switchover request.

```
lorryctl switchover [flags]
```

### Examples

```

lorryctl switchover  --primary xxx --candidate xxx
  
```

### Options

```
  -c, --candidate string    The candidate pod name
  -f, --force               force to swithover if failed
  -h, --help                Print this help message
      --lorry-addr string   The addr of lorry to request (default "http://localhost:3501/v1.0/")
  -p, --primary string      The primary pod name
```

### Options inherited from parent commands

```
      --kb-runtime-dir string   The directory of kubeblocks binaries (default "/kubeblocks/")
```

### SEE ALSO

* [lorryctl](lorryctl.md)	 - LORRY CLI

#### Go Back to [LorryCtl Overview](cli.md) Homepage.

