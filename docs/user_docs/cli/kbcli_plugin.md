---
title: kbcli plugin
---

Provides utilities for interacting with plugins.

### Synopsis

Provides utilities for interacting with plugins.

 Plugins provide extended functionality that is not part of the major command-line distribution.

### Options

```
  -h, --help   help for plugin
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "$HOME/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
      --disable-compression            If true, opt-out of response compression for all requests to the server
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO


* [kbcli plugin describe](kbcli_plugin_describe.md)	 - Describe a plugin
* [kbcli plugin index](kbcli_plugin_index.md)	 - Manage custom plugin indexes
* [kbcli plugin install](kbcli_plugin_install.md)	 - Install kbcli or kubectl plugins
* [kbcli plugin list](kbcli_plugin_list.md)	 - List all visible plugin executables on a user's PATH
* [kbcli plugin search](kbcli_plugin_search.md)	 - Search kbcli or kubectl plugins
* [kbcli plugin uninstall](kbcli_plugin_uninstall.md)	 - Uninstall kbcli or kubectl plugins
* [kbcli plugin upgrade](kbcli_plugin_upgrade.md)	 - Upgrade kbcli or kubectl plugins

#### Go Back to [CLI Overview](cli.md) Homepage.

