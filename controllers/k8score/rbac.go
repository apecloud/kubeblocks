/*
Copyright 2022 The KubeBlocks Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8score

// full access on core API resources
//+kubebuilder:rbac:groups=core,resources=secrets;configmaps;services;resourcequotas;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=core,resources=services/status;resourcequotas/status;persistentvolumeclaims/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=services/finalizers;secrets/finalizers;configmaps/finalizers;resourcequotas/finalizers;persistentvolumeclaims/finalizers,verbs=update

// read + update access
//+kubebuilder:rbac:groups=core,resources=pods;endpoints,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// read only + watch access
//+kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch

// events API only allows ready-only, create, patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update
