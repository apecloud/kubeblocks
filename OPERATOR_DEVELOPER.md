# Operator Developer README

## Terminology

Operator - 自定义控制器，通常安装在 K8s 集群内部，操作权限限制于 ServiceAccount token

Operand - 被 Operator 服务管理管理的工作负载

Custom Resource Definition (CRD) - 自定义 API 声明，提供 CR 对象的蓝图及校验规则

Custom Resource (CR) - 一个 CRD 的实例，可以是一个 Operand 或 操作功能在一个 Operand (a.k.a. primary resources)

Managed resources - Operator 需要管理的 K8s 资源对象(s) (a.k.a. secondary resources)



## Create API with controller

```
# variables
API_GROUP=dac
# API_VERSION name scheme v<N>[alpha|beta<N>]
API_VERSION=v1alpha1
API_NAME=DatabaseInstance    

# 第一个 API 创建会生成 `PROJECT` 文件，之后有新的 API 定义将会扩展到 `PROJECT` 文件
kubebuilder create api --group ${API_GROUP} --version ${API_VERSION} --kind ${API_NAME} --controller --resource 
make manifests


## 以下文件会被创建
api/databaseinstance_types.go
api/groupversion_info.go
api/zz_generated.deepcopy.go
controllers/databaseinstance_controller.go
controllers/suite_test.go

config/crd/bases/dac.infracreate.com_databaseinstances.yaml
config/crd/patches/cainjection_in_databaseinstances.yaml
config/crd/patches/webhook_in_databaseinstances.yaml
config/rbac/databaseinstance_editor_role.yaml
config/rbac/databaseinstance_viewer_role.yaml
config/samples/dac_v1alpha1_databaseinstance.yaml


## main.go 会在 //+kubebuilder:scaffold:builder marker 前面加入以下

+       if err = (&controllers.DatabaseInstanceReconciler{
+               Client: mgr.GetClient(),
+               Scheme: mgr.GetScheme(),
+       }).SetupWithManager(mgr); err != nil {
+               setupLog.Error(err, "unable to create controller", "controller", "DatabaseInstance")
+               os.Exit(1)
+       }
```


## Create Validating Webhook (Optional)

```
kubebuilder create webhook --group ${API_GROUP}  --version ${API_VERSION} --kind ${API_NAME} --defaulting --programmatic-validation
make manifests


## 以下文件会被创建
api/v1alpha1/databaseinstance_webhook.go
api/v1alpha1/webhook_suite_test.go
config/certmanager/certificate.yaml
config/default/manager_webhook_patch.yaml
config/default/webhookcainjection_patch.yaml
config/webhook/service.yaml
config/webhook/manifests.yaml

## main.go 会在 //+kubebuilder:scaffold:builder marker 前面加入以下

+       if err = (&dacv1alpha1.DatabaseInstance{}).SetupWebhookWithManager(mgr); err != nil {
+               setupLog.Error(err, "unable to create webhook", "webhook", "DatabaseInstance")
+               os.Exit(1)
+       }
```

## Enable Webhook testing (optional)

```shell
make run ENABLE_WEBHOOKS=true
```

## Implementing a controller
### Concepts:
#### API design
- Is object scope namespace or cluster scope? (`//+kubebuilder:resource:scope=[Namespaced(default)|Cluster]`)
- Shift-Left principle, DO VALIDATIONS with [CRD Validation](https://book.kubebuilder.io/reference/markers/crd-validation.html)
  and/or with [defaulting/validating webhooks](https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
- Provides enough information for (`kubectl get <kind>`) with CRD's printcolumn marker (`//+kubebuilder:printcolumn`)
- Do replicas handling with [Scale subresources](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource) 
  (`//+kubebuilder:subresource:scale:selectorpath=<string>,specpath=<string>,statuspath=<string>`), 
  as this will enables the "/scale" subresource on a CRD.

#### Controller reconciling handling
Start writing your operator procedures, following provides a guideline
on reconciling procedures:
1. get object, do check object's existence.
2. handles deletion and attach finalizer.
3. reconciling processed state handling ASAP:
  - checked .status.observedGeneration.
  - checked .status.phase is in an unrecoverable state.
4. emit events ("Events" object) for state changes on operand. (Comply with Operator Capability Level 4)
5. read [Working with Kubernetes Objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/).

#### Know your stuff
- controller-runtime client default's configuration enable caches, i.e., r.Client.Get(), r.Client.List()
  always read from caches, check (`go doc sigs.k8s.io/controller-runtime/pkg/manager.Options.NewCache`) if default cache function doesn't suit your needs.
- kubebuilder [`PROJECT` config file](https://book.kubebuilder.io/reference/project-config.html) 


#### Other tips:
- use Patch pattern (`r.Client.Patch()`), for updating .metadata.annotations .metadata.labels and for .status (`r.Client.Status().Patch()`)
- carefully design keys for annotations, labels, and finalizers
- r.Client.List() without filtering scope is EVIL! Always use label matching 
  filters - 'client.MatchingLabelsSelector/client.MatchingLabels{}/client.HasLabels[]'
  and apply Namespace scope whenever applicable.
- for complicated states transition handling, do consider employ [state pattern](https://refactoring.guru/design-patterns/state), check [stateless](https://pkg.go.dev/github.com/qmuntal/stateless#readme-export-to-dot-graph).


# References:
## Kubebuilder
- [Kubebuilder book](https://book.kubebuilder.io/introduction.html)
- [Default Exported Metrics References](https://book.kubebuilder.io/reference/metrics-reference.html)

## Helm
- [Built-in Objects](https://helm.sh/docs/chart_template_guide/builtin_objects/)
- [Template Function List](https://helm.sh/docs/chart_template_guide/function_list/)

## Operator Stuffs
- [Operator Best Practices](https://sdk.operatorframework.io/docs/best-practices/best-practices/)
- [Operator Capability Levels](https://sdk.operatorframework.io/docs/overview/operator-capabilities/)