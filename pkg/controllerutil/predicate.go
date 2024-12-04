/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package controllerutil

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	//         pkg                   reconciler               resource                 sub-resources             operation
	// experimentalv1alpha1 NodeCountScalerReconciler      NodeCountScaler          corev1.Node                      w
	//                                                                              appsv1alpha1.Cluster             w
	// extensionsv1alpha1   AddonReconciler                Addon                    batchv1.Job                      w
	// corev1               EventReconciler                Event
	// workloadsv1alpha1    InstanceSetReconciler          InstanceSet              corev1.Pod                       w
	//                                                                              corev1.PersistentVolumeClaim     o
	//	                                                                            batchv1.Job                      o
	//		                                                                        corev1.Service                   o
	//		                                                                        corev1.ConfigMap                 o
	// appsv1beta1          ConfigConstraintReconciler     ConfigConstraint         corev1.ConfigMap                 o
	// appsv1alpha1         OpsRequestReconciler           OpsRequest  		        appsv1alpha1.Cluster             w
	//		                                                                        workloadsv1alpha1.InstanceSet    w
	//																	            dpv1alpha1.Backup                w
	//																	            corev1.PersistentVolumeClaim     w
	//																	            corev1.Pod                       w
	//																	            batchv1.Job                      o
	//																	            dpv1alpha1.Restore               o
	//                      ReconfigureReconciler 		   corev1.ConfigMap
	//                      ConfigurationReconciler 	   Configuration            corev1.ConfigMap                 o
	//                      ClusterReconciler 			   Cluster                  appsv1alpha1.Component           o
	//																                corev1.Service                   o
	//																                corev1.Secret                    o
	//																                dpv1alpha1.BackupPolicy          o
	//																                dpv1alpha1.BackupSchedule        o
	//                      SystemAccountReconciler 	   Cluster                  corev1.Secret                    o
	//																	            batchv1.Job                      w
	//                      ComponentReconciler 		   Component                workloads.InstanceSet            o
	//																                corev1.Service                   o
	//																                corev1.Secret                    o
	//																	            corev1.ConfigMap                 o
	//																	            dpv1alpha1.Backup                o
	//																	            dpv1alpha1.Restore               o
	//																	            corev1.PersistentVolumeClaim     w
	//																	            batchv1.Job                      o
	//																	            appsv1alpha1.Configuration       w
	//      															            rbacv1.ClusterRoleBinding        o/w
	//																	            rbacv1.RoleBinding               o/w
	//																	            corev1.ServiceAccount            o/w
	//                      BackupPolicyTemplateReconciler BackupPolicyTemplate     appsv1alpha1.ComponentDefinition w
	//                      ComponentClassReconciler 	   ComponentClassDefinition
	//                      ClusterVersionReconciler 	   ClusterVersion
	//                      ServiceDescriptorReconciler    ServiceDescriptor
	//                      ClusterDefinitionReconciler    ClusterDefinition
	//                      OpsDefinitionReconciler 	   OpsDefinition
	//                      ComponentDefinitionReconciler  ComponentDefinition
	//                      ComponentVersionReconciler 	   ComponentVersion 	    appsv1alpha1.ComponentDefinition w
	//
	// has new version： - filter by api version label/annotation
	//    addon: ClusterDefinition, ComponentDefinition, ComponentVersion, BackupPolicyTemplate
	//	  user：ServiceDescriptor, Cluster
	//    controller: Component, InstanceSet
	// unchanged：NodeCountScaler, Addon - the new operator will be responsible for these
	// deleted：ClusterVersion, ComponentClassDefinition - nothing to do
	// group changed：OpsRequest, OpsDefinition, ConfigConstraint, Configuration - nothing to do
	// TODO:
	//    EventReconciler.Event

	managedNamespaces       *sets.Set[string]
	supportedCRDAPIVersions = sets.New[string](
		// ClusterDefinition, ComponentDefinition, ComponentVersion, Cluster, Component
		appsv1.GroupVersion.String(),
		// InstanceSet
		workloadsv1.GroupVersion.String(),
	)
)

func IsSupportedCRDAPIVersion(apiVersion string) bool {
	return supportedCRDAPIVersions.Has(apiVersion)
}

func NewControllerManagedBy(mgr manager.Manager, objs ...client.Object) *builder.Builder {
	b := ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicate.NewPredicateFuncs(namespacePredicateFilter))
	if len(objs) > 0 {
		b.WithEventFilter(predicate.NewPredicateFuncs(newAPIVersionPredicateFilter(objs)))
	}
	return b
}

func namespacePredicateFilter(object client.Object) bool {
	if managedNamespaces == nil {
		set := &sets.Set[string]{}
		namespaces := viper.GetString(strings.ReplaceAll(constant.ManagedNamespacesFlag, "-", "_"))
		if len(namespaces) > 0 {
			set.Insert(strings.Split(namespaces, ",")...)
		}
		managedNamespaces = set
	}
	if len(*managedNamespaces) == 0 || len(object.GetNamespace()) == 0 {
		return true
	}
	return managedNamespaces.Has(object.GetNamespace())
}

func newAPIVersionPredicateFilter(objs []client.Object) func(client.Object) bool {
	return func(obj client.Object) bool {
		annotations := obj.GetAnnotations()
		if annotations != nil {
			apiVersion, ok := annotations[constant.CRDAPIVersionAnnotationKey]
			if ok {
				return IsSupportedCRDAPIVersion(apiVersion)
			}
		}
		if reflect.TypeOf(obj) == reflect.TypeOf(&appsv1.Cluster{}) {
			return true // to resolve the CRD API version of the cluster
		}
		for _, wobj := range objs {
			if reflect.TypeOf(obj) == reflect.TypeOf(wobj) {
				return true // watched objects, but has no CRD API version, it may be the old version
			}
		}
		return true
	}
}
