<h1>tpltool</h1>

# 1. Introduction

Welcome to tpltool - a developer tool integrated with Kubeblocks that can help developers quickly generate rendered configurations or scripts based on Helm templates, and discover errors in the template before creating the database cluster.

# 2. Getting Started

You can get started with tpltool, by any of the following methods:
* Build `reloader` from sources

## 2.1 Build

Compiler `Go 1.19+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `make tpltool` to build and produce the `tpltool` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks
$ make tpltool
```

## 2.2 Run

You can run the following command to start tpltool once built

```shell
tpltool Provides a mechanism to rendered template for ComponentConfigSpec and ComponentScriptSpec in the ClusterComponentDefinition.

Usage:
  tpltool [flags]

Flags:
  -a, --all                                      template all config/script specs
      --clean                                    specify whether to clear the output dir
      
      # specify the cluster yaml
      --cluster string                           the cluster yaml file
      # specify the clusterdefinition yaml
      --cluster-definition string                the cluster definition yaml file
      
      --component-name string                    specify the component name of the clusterdefinition
      --config-spec string                       specify the config spec to be rendered
      
      # for mock cluster yaml
      --cpu string                               specify the cpu of the component
      --memory string                            specify the memory of the component
      --volume-name string                       specify the data volume name of the component
      
      --helm string                              specify the helm template dir of the component
      --helm-output string                       specify the helm template output dir

  -o, --output-dir string                        specify the output directory
  -r, --replicas int32                           specify the replicas of the component (default 1)
     
```

```shell

# the first way

$ ./bin/tpltool --helm ./deploy/apecloud-mysql --output-dir ./rendered_output --clean --cpu=200 --memory=10G --config-spec mysql-consensusset-config
wrote ./temp_helm_template_output/apecloud-mysql/templates/configmap.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/configmap.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/scripts.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/backuppolicytemplate.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/backuptool.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/clusterdefinition.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/clusterversion.yaml
wrote ./temp_helm_template_output/apecloud-mysql/templates/configconstraint.yaml


2023-04-02T23:25:07+08:00       INFO    tpltool   rendering template:
2023-04-02T23:25:07+08:00       INFO    tpltool   config spec: mysql-consensusset-config, template name: mysql8.0-config-template in the component[mysql]
2023-04-02T23:25:07+08:00       INFO    dump rendering template spec: mysql-consensusset-config, output directory: rendered_output/cluster-HVevhr-mysql-Qsq-mysql-config

$ ls rendered_output/cluster-HVevhr-mysql-Qsq-mysql-config
my.cnf


# the second way
# helm template deploy/apecloud-mysql --output-dir ${helm_template_output}
$ ./bin/tpltool --helm-output ${helm_template_output} -a 

```


# 7. License

Reloader is under the Apache 2.0 license. See the [LICENSE](../../LICENSE) file for details.
