##Procedure 1: You can use kbcli connectcommand and specify the cluster name to be connected.
```
kbcli cluster connect ${cluster-name}
```
The lower-level command is actually kubectl exec. The command is functional as long as the k8s apiserver is accessible.
##Procedure 2: To connect database with CLI or SDK client.
Execute the following command to get the network information of the targeted database and connect it with the printed IP address.
```
kbcli cluser connect --show-example ${cluster-name}
```
Information printed includes database addresses, port No., username, password.  The figure below is an example of MySQL database network information.
- Address: -h specifies the server address . In the example below it is 127.0.0.1
- Port: -P specifies port No. , In the example below it is 3306.
- User: -u is the user name.
- Password: -p shows the password. In the example below, it is hQBCKZLI. 
>Note: the password does not include -p.

![Example](../../img/connect_database_with_CLI_or_SDK_client.png)