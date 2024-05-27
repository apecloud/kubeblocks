---
title: kbcli alert add-receiver
---

Add alert receiver, such as email, slack, webhook and so on.

```
kbcli alert add-receiver [flags]
```

### Examples

```
  # add webhook receiver without token, for example feishu
  kbcli alert add-receiver --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo'
  
  # add webhook receiver with token, for example feishu
  kbcli alert add-receiver --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=XXX'
  
  # add email receiver
  kbcli alert add-receiver --email='user1@kubeblocks.io,user2@kubeblocks.io'
  
  # add email receiver, and only receive alert from cluster mycluster
  kbcli alert add-receiver --email='user1@kubeblocks.io,user2@kubeblocks.io' --cluster=mycluster
  
  # add email receiver, and only receive alert from cluster mycluster and alert severity is warning
  kbcli alert add-receiver --email='user1@kubeblocks.io,user2@kubeblocks.io' --cluster=mycluster --severity=warning
  
  # add slack receiver
  kbcli alert add-receiver --slack api_url=https://hooks.slackConfig.com/services/foo,channel=monitor,username=kubeblocks-alert-bot
```

### Options

```
      --cluster stringArray    Cluster name, such as mycluster, more than one cluster can be specified, such as mycluster1,mycluster2
      --email stringArray      Add email address, such as user@kubeblocks.io, more than one emailConfig can be specified separated by comma
  -h, --help                   help for add-receiver
      --severity stringArray   Alert severity level, critical, warning or info, more than one severity level can be specified, such as critical,warning
      --slack stringArray      Add slack receiver, such as api_url=https://hooks.slackConfig.com/services/foo,channel=monitor,username=kubeblocks-alert-bot
      --webhook stringArray    Add webhook receiver, such as url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=xxxxx
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

* [kbcli alert](kbcli_alert.md)	 - Manage alert receiver, include add, list and delete receiver.

#### Go Back to [CLI Overview](cli.md) Homepage.

