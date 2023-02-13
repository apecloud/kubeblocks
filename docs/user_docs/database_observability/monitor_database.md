# Monitor database
KubeBlocks provides database monitoring, management, and obervability solution for MySQL. You can observe the health of your database systems, and track and measure your database in real-time to optimize application performance.
With KubeBlocks, you can set the monitor without a complicated yaml file, and a consistent experience is supported with the same kbcli command line.

## Before you start

Install monitor component by setting *`monitor`* parameter value as *`true`* when installing KubeBlocks with kbcli.

*Example*
```
kbcli kubeblocks install --monitor=true
```
## Enable monitor function
- When creating clusters, set the `Monitor` parameter value as *`true`*. The monitor function is enabled.
By default, the `Monitor` parameter value is *`false`*.

*Example*
```
 kbcli cluster create mycluster --set-file=http://kubeblocks.io/yamls/apecloud-mysql-single.yaml --termination-policy=WipeOut --monitor=true
```
- For existed clusters, tbd.

### For configure Grafana, Ask Lingce!