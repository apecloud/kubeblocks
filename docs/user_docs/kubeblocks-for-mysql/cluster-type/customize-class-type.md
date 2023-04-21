# Customize class types

You can use KubeBlocks to create your own class types.

***Steps:***

- To create one class.

1. Use ```kbcli class create``` to create customized class types.

   *Example:*

   ```
   kbcli class create custom-1c2g --cluster-definition apecloud-mysql --type mysql --constraint kb-resource-constraint-general --cpu 1 --memory 1Gi --storage name=data,size=10Gi
   ```

2. Check whether the class is created with ```kbcli class list``` command.

    ```
    kbcli class list --cluster-definition apecloud-mysql  
    ```

    And you can see a class named `custom-1c2g` is listed.
       
-To create many classes at a time

1. If you want to create many classes, you can use yaml file.
For example, you can create the file named `/tmp/class.yaml`.

```
- resourceConstraintRef: kb-resource-constraint-general
  # class template, you can declare variables and set default values here
  template: |
    cpu: "{{ or .cpu 1 }}"
    memory: "{{ or .memory 4 }}Gi"
    volumes:
    - name: data
      size: "{{ or .dataStorageSize 10 }}Gi"
    - name: log
      size: "{{ or .logStorageSize 1 }}Gi"
  # template variables used to define classes
  vars: [cpu, memory, dataStorageSize, logStorageSize]
  series:
  - # class naming template, you can reference variables in class template
    # it's also ok to define static class name in the following class definitions
    namingTemplate: "custom-{{ .cpu }}c{{ .memory }}g"

    # class definitions, we support two kinds of class definitions:
    # 1. define values for template variables and the full class definition will be dynamically rendered
    # 2. statically define the complete class
    classes:
    - args: [1, 4, 1, 1]
    - args: [1, 6, 1, 1]
```

2. Apply the yaml file to create classes.

```
kbcli class create --cluster-definition apecloud-mysql --type mysql --file /tmp/class.yaml
```

3. Check whether these classes are created with ```kbcli class list``` command.

