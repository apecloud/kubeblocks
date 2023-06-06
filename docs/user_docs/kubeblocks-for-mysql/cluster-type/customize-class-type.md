---
title: Customize class types
description: How to customize class types for a MySQL cluster
keywords: [mysql, class type, customize class types]
sidebar_position: 2
sidebar_label: Customize class types
---

# Customize class types

You can use KubeBlocks to create your own class types.

## Create one class

***Steps:***

1. Use `kbcli class create` to create customized class types.

   *Example:*

   ```bash
   kbcli class create custom-1c1g --cluster-definition apecloud-mysql --type mysql --constraint kb-resource-constraint-general --cpu 1 --memory 1Gi
   ```

2. Check whether the class is created with `kbcli class list` command.

    ```bash
    kbcli class list --cluster-definition apecloud-mysql  
    ```

    And you can see a class named `custom-1c1g` is listed.

## Create many classes at a time

***Steps:***

1. If you want to create many classes at a time, you can use yaml file.
   For example, you can create the file named `/tmp/class.yaml`.

    ```bash
    - resourceConstraintRef: kb-resource-constraint-general
      # class template, you can declare variables and set default values here
      template: |
        cpu: "{{ or .cpu 1 }}"
        memory: "{{ or .memory 4 }}Gi"
      # template variables used to define classes
      vars: [cpu, memory]
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

    ```bash
    kbcli class create --cluster-definition apecloud-mysql --type mysql --file /tmp/class.yaml
    ```

3. Check whether these classes are created with `kbcli class list` command.

   ```bash
   kbcli class list --cluster-definition apecloud-mysql  
   ```
