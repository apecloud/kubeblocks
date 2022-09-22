# ISV Configuration Developer README

## Overview

Configuration Tpl是基于go模版库[sprig](https://github.com/Masterminds/sprig)开发的，专门为数据库实例动态配置渲染而开发的工具，ISV可以基于Configuration Tpl编写特定引擎的配置模版.

## Prerequisites

1. 熟悉[Go Template语法](https://pkg.go.dev/text/template#pkg-overview).
2. 对helm有一定了解， 熟悉[Built-in Objects](https://helm.sh/docs/chart_template_guide/builtin_objects/)和[Built-in Functions](https://helm.sh/docs/chart_template_guide/function_list/)的用法，以及如何使用.
3. 对DBaaS的架构有一定了解，熟悉ClusterDefinition，Cluster和Component的定义.

## Configuration Tpl Built-in Function List

通过模版引擎，在配置模版中可以直接使用DB实例的meta信息，比如db实例的规格(Resource)，副本数量，db类型等等.

如果对go template或者helm比较了解，就会知道Built-in Object实际上是比较简单的，是go语言的一个对象，这个对象可以是一个字符串，或者一个数据类型实例. 比如 Cluster这个对象对应的是一个DB实例集群的meta，可以通过Cluster.Name获取到db的实例名称；也可以通过PodSpec获取pod相关的信息，比如Container的资源，通过cpu和memory信息渲染线程数量或者pool buffer的大小；


Configuration Tpl提供了如下的最上层的built-in Objects，这些对象都可以直接在配置模版中使用.

* Cluster:  Cluster描述了数据库集群信息， 可以参考[ClusterSpec](../../apis/dbaas/v1alpha1/cluster_types.go)的定义
    * Cluster.Name: 数据库db的名称
    * Cluster.Namespace: cluster所属的namespace
    * Cluster.Spec.Components[*]: 数据库实例集群所有的Component
     
* Version: Cluster每个组件使用的版本信息, 可以参考[AppVersionSpec](../../apis/dbaas/v1alpha1/appversion_types.go)的定义
    * Version.Spec.Components[*]: 每个Component的Pod和Container的信息，比如mysql的版本号
    * ...

* Component: DBaaS会对每个Component渲染一个或者多个文件，Component是当前需要渲染配置的组件, 参考[Component](../../apis/dbaas/v1alpha1/type.go)的定义
    * Component.Name: Component的名称
    * Component.Type: Component的类型
    * Component.Service: Component  service相关的信息
    * Component.VolumeClaimTemplates: 获取db实例pvc相关的信息
    * ...

* PodSpec: 这个是Component生成的PodTemplate相关的信息，最终会通过这个生成K8S的Pod，类型定义可以参考[K8S PodSpec](https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/api/core/v1/types.go) 的定义
    * PodSpec.Volumes: pvc相关的信息 
    * PodSpec.Containers[*]: 容器相关的信息
      * PodSpec.Containers[*].Ports: 端口相关的信息, 比如，在配置模版中可以通过这个信息获取mysql的服务端口
      * PodSpec.Containers[*].Env: db实例启动的环境变量
      * PodSpec.Containers[*].Resources: db实例启动的资源， 配置渲染一个比较重要的事情就是根据数据库实例的规格，生成相关的pool 和thread num，编写db模版引擎的时候就可以通过自动按照规格渲染
      * PodSpec.Containers[*].VolumeMounts: pod mount的volume信息, 在配置模版中通过volume信息生成日志或者数据路径.
      * ...

* RoleGroup: 这个是replica相关的信息，可以参考 [RoleGroup](../../apis/dbaas/v1alpha1/type.go)的定义
    * RoleGroup.Name: 
    * RoleGroup.Type:
    * RoleGroup.Replicas:
    * RoleGroup.MinAvailable:
    * RoleGroup.MaxAvailable:
    * ...


## Configuration Tpl Built-in Object List

Configuration Tpl集成了go template的模版函数，主要包括如下类型，具体每种类型的接口可以参考[helm的Template Function List文档](https://helm.sh/docs/chart_template_guide/function_list/):

* Math
* List
* String
* Regex
* URL
* Date
* Dictionaries
* Encoding
* Fle Path
* ...

除了上面这些go template内置的函数，Configuration Tpl后续会把helm内置的几个函数也添加进来:

* [include](https://helm.sh/docs/howto/charts_tips_and_tricks/#using-the-include-function) 
* [tpl](https://helm.sh/docs/howto/charts_tips_and_tricks/#using-the-tpl-function)
* [required](https://helm.sh/docs/howto/charts_tips_and_tricks/#using-the-required-function)
* [lookup](https://helm.sh/docs/chart_template_guide/functions_and_pipelines/#using-the-lookup-function)

Configuration Tpl 也提供了一些Built-in function，主要是为了让ISV编写配置模版更方便，目前主要提供了两类函数：

1. General Function, 对Container信息的抽取:
   * get_volume_path_by_name: 获取volumeMount的路径, e.g: 在生产mysql配置的时候，需要知道数据卷的路径，就可以通过下面的function来获取
   
    `(get_volume_path_by_name $container "data")`

    mysql的配置模版生产datadir optional的逻辑如下：
```
    #for my.cnf render
    
    {{- $data_root := get_volume_path_by_name ( index .PodSpec.Containers 0 ) "data" }}
    # render datadir optional
    datadir={{ $data_root }}/data
```
   
   * get_pvc_by_name
   * get_env_by_name
   * get_port_by_name
   * get_container_by_name

2. Specific engine functions: 为了简化配置模版的复杂的，对不同引擎提供了一些built-in function来简化引擎参数的生成逻辑

   * call_buffer_size_by_resource: 根据container的资源定义，计算pool buffer推荐值 
   
    这个场景主要是考虑mysql根据规格计算pool比较麻烦，参考aws/aliyun的不同规格的buffer推荐参数，也参考了mysql文档中对相关参数的限制性约束，提供的一个built-in function, 接口使用如下：
 
   ` call_buffer_size_by_resource $container_object `
 
   mysql的配置模版中的使用场景:
```
    #for my.cnf render
    
    {{- $pool_buffer_size := ( call_buffer_size_by_resource ( index .PodSpec.Containers 0 ) ) }}

    {{- if $pool_buffer_size }}
    innodb-buffer-pool-size={{ $pool_buffer_size }}
    {{- end }}
```