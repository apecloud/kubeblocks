1. Enable the addon postgresql and redis.
    ```shell
    kbcli addon enable postgresql
    kbcli addon enable redis
    ```
2. When enabled successfully, you can check it with ```kbcli addon list``` and ```kubectl get clusterdefinition```.
   ```shell
   kbcli addon list 
   kubectl get clusterdefinition
   NAME             MAIN-COMPONENT-NAME   STATUS      AGE
   redis            redis                 Available   6h43m
   postgresql       postgresql            Available   6h42m
   ```
3. Install the delphic with helm.
   ```shell
   # TODO publish the delphic to public helm repository
   helm install delphic ./deploy/delphic
   ```
4. Check whether the plugin is installed successfully.
   ```
   kubectl get pods
   NAME                               READY   STATUS      RESTARTS      AGE
   delphic-redis-redis-0              3/3     Running     0             42m
   delphic-redis-redis-1              3/3     Running     0             42m
   delphic-redis-redis-sentinel-0     1/1     Running     1 (41m ago)   42m
   delphic-redis-redis-sentinel-2     1/1     Running     1 (41m ago)   42m
   delphic-redis-redis-sentinel-1     1/1     Running     1 (41m ago)   42m
   delphic-6f747fb8f7-4hdh5           5/5     Running     0             43m
   delphic-create-django-user-lmpnq   1/1     Completed   1             43m
   delphic-postgres-postgresql-0      4/4     Running     0             42m
   delphic-postgres-postgresql-1      4/4     Running     0             42m
   ```
5. Find the username and password of web console.
   On Mac OS X:
   ```
   kubectl get secret delphic-django-secret -o jsonpath='{.data.username}' | base64 -D
   kubectl get secret delphic-django-secret -o jsonpath='{.data.password}' | base64 -D
   ```
   
   On Linux:
   ```
   kubectl get secret delphic-django-secret -o jsonpath='{.data.username}' | base64 -d
   kubectl get secret delphic-django-secret -o jsonpath='{.data.password}' | base64 -d
   ```
    
6. Port-forward the Plugin Portal to access it.
   ```shell
   kubectl port-forward port-forward deployment/delphic 3000:3000 8000:8000
   ```
7. In your web browser, open the plugin portal with the address ```http://127.0.0.1:3000```