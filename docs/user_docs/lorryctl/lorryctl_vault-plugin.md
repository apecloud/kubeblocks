---
title: lorryctl vault-plugin
---

run a vault-plugin service.

```
lorryctl vault-plugin [flags]
```

### Examples

```

lorryctl vault-plugin  --primary xxx --candidate xxx
  
```

### Options

```
  -c, --candidate string    The candidate pod name
  -h, --help                Print this help message
      --lorry-addr string   The addr of lorry to request (default "localhost:3501")
  -l, --primary string      The primary pod name
```

### Options inherited from parent commands

```
      --kb-runtime-dir string   The directory of kubeblocks binaries (default "/kubeblocks/")
```

### SEE ALSO

* [lorryctl](lorryctl.md)	 - LORRY CLI

#### Go Back to [LorryCtl Overview](cli.md) Homepage.

