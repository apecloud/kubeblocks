## Installing add-ons
An add-on is software that provides supporting operational capabilities to Kubernetes applications.
By default, all add-ons supported are automatically installed.
To list supported add-ons. Execute```kbcli addon list ```command.
**Example**
```
~ kbcli addon list
NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
alertmanager-webhook-adaptor   Helm   Enabled                   true
apecloud-mysql                 Helm   Enabled                   true
csi-s3                         Helm   Enabled                   true
grafana                        Helm   Enabled                   true
loadbalancer                   Helm   Enabled   agent           true          {key=KubeGitVersion,op=Contains,values=[eks]}
postgresql                     Helm   Enabled                   true
prometheus                     Helm   Enabled    alertmanager   true
snapshot-controller            Helm   Enabled                   true
```
> Note: Some add-ons have a requirement for environment. 
If the certain requirement is not meet, the automatic installation is invalid.

You can perform the following steps to check and install the add-on.

Steps:
1. Run ```kbcli addon describe```, and check the *Installable* part.
**Example**
```
kbcli addon describe snapshot-controller
>
Name:       snapshot-controller
Labels:     kubeblocks.io/provider=community
Type:       helm
Extras:     
Status:     disabled
Default Install Info:
  Version:  15.16
  Name          Replicas  CPU(req/limit)  Memory(req/limit)  Storage
  ----          --------  --------------  -----------------  -------
  main                 1               /                  /        -
Installable:  kubeGitVersion |= [eks,ack]
Auto-Install: true

Events:     <none>
>
```
The installable part says when the kubeGitVersion content includes *eks* and *ack*, the auto-install is enabled.
In this case, you can check the version of kubernetes cluster, use the following command.
```
kubectl version -ojson | jq '.serverVersion.gitVersion'
>
"v1.24.4+eks"
>
```
As the printed output suggested, *eks* is included. And you can go on with the next step. In case that eks is not included, it is invalid to enable the add-on.

2. To enable the add-on, use ```kbcli addon enable```.
**Example**
```
kbcli addon enable snapshot-controller
>
"snapshot-controller" addon enabled.
>
```
3. List the add-ons to check whether it is enabled.

```
kbcli addon list
>
NAME                 TYPE  STATUS    EXTRAS        INSTALLABLE-SELECTOR                               AUTO-INSTALL
prometheus           helm  enabled   alertmanager  
apecloud-mysql       helm  disabled                
csi-s3               helm  disabled                {key=KubeGitVersion,op=Contains,values=[eks,ack]}  false
grafana              helm  disabled                
loadbalancer         helm  disabled                {key=KubeGitVersion,op=Contains,values=[eks]}      true
snapshot-controller  helm  enabled                 {key=KubeGitVersion,op=Contains,values=[eks,ack]}  true
>
```
