---
title: lorryctl revoke-role
---

revoke user role.

```
lorryctl revoke-role [flags]
```

### Examples

```

lorryctl  revoke-role --username xxx --rolename xxx 
  
```

### Options

```
  -h, --help                Print this help message
      --lorry-addr string   The addr of lorry to request (default "http://localhost:3501/v1.0/")
      --rolename string     The name of role to revoke
      --username string     The name of user to revoke
```

### Options inherited from parent commands

```
      --kb-runtime-dir string   The directory of kubeblocks binaries (default "/kubeblocks/")
```

### SEE ALSO

* [lorryctl](lorryctl.md)	 - LORRY CLI

#### Go Back to [LorryCtl Overview](cli.md) Homepage.

