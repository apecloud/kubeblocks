---
title: sqlctl switchover
---

execute a switchover request.

```
sqlctl switchover [flags]
```

### Examples

```

sqlctl switchover  --primary xxx --candidate xxx
  
```

### Options

```
  -c, --candidate string         The candidate pod name
  -h, --help                     Print this help message
  -l, --primary string           The primary pod name
      --sqlchannel-addr string   The addr of sqlchannel to request (default "localhost:3501")
```

### Options inherited from parent commands

```
      --kb-runtime-dir string   The directory of kubeblocks binaries (default "/kubeblocks/")
```

### SEE ALSO

* [sqlctl](sqlctl.md)	 - SQL Channel CLI

#### Go Back to [SQLCtl Overview](cli.md) Homepage.

