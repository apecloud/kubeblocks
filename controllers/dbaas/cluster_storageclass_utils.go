/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// handleClusterVolumeExpansion when StorageClass changed, we should handle the PVC of cluster whether volume expansion is supported
func handleClusterVolumeExpansion(reqCtx intctrlutil.RequestCtx, cli client.Client, storageClass *storagev1.StorageClass) error {
	var err error
	clusterList := &dbaasv1alpha1.ClusterList{}
	if err = cli.List(reqCtx.Ctx, clusterList); err != nil {
		return err
	}
	// handle the created cluster
	storageCLassName := storageClass.Name
	for _, cluster := range clusterList.Items {
		// if cluster not used the StorageClass, continue
		if !clusterContainsStorageClass(&cluster, storageCLassName) {
			continue
		}
		patch := client.MergeFrom(cluster.DeepCopy())
		if needPatchClusterStatusOperations, err := needSyncClusterStatusOperations(reqCtx, cli, &cluster, storageClass); err != nil {
			return err
		} else if !needPatchClusterStatusOperations {
			continue
		}
		if err = cli.Status().Patch(reqCtx.Ctx, &cluster, patch); err != nil {
			return err
		}
	}
	return nil
}

// needSyncClusterStatusOperations check cluster whether sync status.operations.volumeExpandable
func needSyncClusterStatusOperations(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, storageClass *storagev1.StorageClass) (bool, error) {
	// get cluster pvc list
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  cluster.GetName(),
		intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(reqCtx.Ctx, pvcList, inNS, ml); err != nil {
		return false, err
	}
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &dbaasv1alpha1.Operations{}
	}
	// if no pvc, do it
	if len(pvcList.Items) == 0 {
		return handleNoExistsPVC(reqCtx, cli, cluster)
	}
	var (
		needSyncStatusOperations bool
		// save the handled pvc
		handledPVCMap = map[string]struct{}{}
	)
	for _, v := range pvcList.Items {
		if *v.Spec.StorageClassName != storageClass.Name {
			continue
		}
		componentName := v.Labels[intctrlutil.AppComponentLabelKey]
		volumeClaimTemplateName := getVolumeClaimTemplateName(v.Name, cluster.Name, componentName)
		componentVolumeClaimName := fmt.Sprintf("%s-%s", componentName, volumeClaimTemplateName)
		if _, ok := handledPVCMap[componentVolumeClaimName]; ok {
			continue
		}
		// check whether volumeExpandable changed, then sync cluster.status.operations
		if needSync := needSyncClusterStatus(storageClass, componentName, volumeClaimTemplateName, cluster); needSync {
			needSyncStatusOperations = true
		}
		handledPVCMap[componentVolumeClaimName] = struct{}{}
	}
	return needSyncStatusOperations, nil
}

// clusterContainsStorageClass check whether cluster used the StorageClass
func clusterContainsStorageClass(cluster *dbaasv1alpha1.Cluster, storageClassName string) bool {
	if cluster.Annotations == nil {
		return false
	}
	storageClassNameList := strings.Split(cluster.Annotations[intctrlutil.StorageClassAnnotationKey], ",")
	return slices.Contains(storageClassNameList, storageClassName)
}

func isDefaultStorageClassAnnotation(storageClass *storagev1.StorageClass) bool {
	return storageClass.Annotations != nil && storageClass.Annotations[storage.IsDefaultStorageClassAnnotation] == "true"
}

func isSupportVolumeExpansion(storageClass *storagev1.StorageClass) bool {
	return storageClass.AllowVolumeExpansion != nil && *storageClass.AllowVolumeExpansion
}

// getSupportVolumeExpansionComponents Get the components that support volume expansion and the volumeClaimTemplates
func getSupportVolumeExpansionComponents(ctx context.Context, cli client.Client,
	cluster *dbaasv1alpha1.Cluster) ([]dbaasv1alpha1.OperationComponent, error) {
	var (
		storageClassMap             = map[string]bool{}
		hasCheckDefaultStorageClass bool
		// the default storageClass may not exist, so use a bool key to check
		defaultStorageClassAllowExpansion bool
		volumeExpansionComponents         = make([]dbaasv1alpha1.OperationComponent, 0)
	)
	for _, v := range cluster.Spec.Components {
		operationComponent := dbaasv1alpha1.OperationComponent{}
		for _, vct := range v.VolumeClaimTemplates {
			if vct.Spec == nil {
				continue
			}
			// check the StorageClass whether support volume expansion
			if ok, err := checkStorageClassIsSupportExpansion(ctx, cli, storageClassMap, vct.Spec.StorageClassName,
				&hasCheckDefaultStorageClass, &defaultStorageClassAllowExpansion); err != nil {
				return nil, err
			} else if ok {
				operationComponent.VolumeClaimTemplateNames = append(operationComponent.VolumeClaimTemplateNames, vct.Name)
			}
		}

		if len(operationComponent.VolumeClaimTemplateNames) > 0 {
			operationComponent.Name = v.Name
			volumeExpansionComponents = append(volumeExpansionComponents, operationComponent)
		}
	}
	if len(storageClassMap) > 0 {
		// patch cluster StorageClass annotation
		patch := client.MergeFrom(cluster.DeepCopy())
		if cluster.Annotations == nil {
			cluster.Annotations = map[string]string{}
		}
		keys := maps.Keys(storageClassMap)
		cluster.Annotations[intctrlutil.StorageClassAnnotationKey] = strings.Join(keys, ",")
		if err := cli.Patch(ctx, cluster, patch); err != nil {
			return volumeExpansionComponents, err
		}
	}
	return volumeExpansionComponents, nil
}

// checkStorageClassIsSupportExpansion check whether the storageClass supports volume expansion
func checkStorageClassIsSupportExpansion(ctx context.Context,
	cli client.Client,
	storageClassMap map[string]bool,
	storageClassName *string,
	hasCheckDefaultStorageClass *bool,
	defaultStorageClassAllowExpansion *bool) (bool, error) {
	var (
		ok  bool
		err error
	)
	if storageClassName != nil {
		if ok, err = checkSpecifyStorageClass(ctx, cli, storageClassMap, *storageClassName); err != nil {
			return false, err
		}
		return ok, nil
	} else {
		// get the default StorageClass whether supports volume expansion for the first time
		if !*hasCheckDefaultStorageClass {
			if *defaultStorageClassAllowExpansion, err = checkDefaultStorageClass(ctx, cli, storageClassMap); err != nil {
				return false, err
			}
			*hasCheckDefaultStorageClass = true
		}
		return *defaultStorageClassAllowExpansion, nil
	}
}

// checkStorageClassIsSupportExpansion check whether the specified storageClass supports volume expansion
func checkSpecifyStorageClass(ctx context.Context, cli client.Client, storageClassMap map[string]bool, storageClassName string) (bool, error) {
	var (
		supportVolumeExpansion bool
	)
	if val, ok := storageClassMap[storageClassName]; ok {
		return val, nil
	}
	// if storageClass is not in the storageClassMap, get it
	storageClass := &storagev1.StorageClass{}
	if err := cli.Get(ctx, types.NamespacedName{Name: storageClassName}, storageClass); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	// get bool value of StorageClass.AllowVolumeExpansion and put it to storageClassMap
	if storageClass != nil && storageClass.AllowVolumeExpansion != nil {
		supportVolumeExpansion = *storageClass.AllowVolumeExpansion
	}
	storageClassMap[storageClassName] = supportVolumeExpansion
	return supportVolumeExpansion, nil
}

// checkDefaultStorageClass check whether the default storageClass supports volume expansion
func checkDefaultStorageClass(ctx context.Context, cli client.Client, storageClassMap map[string]bool) (bool, error) {
	storageClassList := &storagev1.StorageClassList{}
	if err := cli.List(ctx, storageClassList); err != nil {
		return false, err
	}
	// check the first default storageClass
	for _, sc := range storageClassList.Items {
		if isDefaultStorageClassAnnotation(&sc) {
			allowExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
			storageClassMap[sc.Name] = allowExpansion
			return allowExpansion, nil
		}
	}
	return false, nil
}

// getVolumeClaimTemplateName get volumeTemplate name by cluster name and component name
func getVolumeClaimTemplateName(pvcName, clusterName, componentName string) string {
	separator := fmt.Sprintf("-%s-%s", clusterName, componentName)
	return strings.Split(pvcName, separator)[0]
}

// needSyncClusterStatusWhenAllowExpansion when StorageClass allow volume expansion, do it
func needSyncClusterStatusWhenAllowExpansion(componentName, volumeClaimTemplateName string, cluster *dbaasv1alpha1.Cluster) bool {
	var (
		needSyncStatusOperations bool
		needAppendComponent      = true
		volumeExpandable         = cluster.Status.Operations.VolumeExpandable
	)

	for i, x := range volumeExpandable {
		if x.Name != componentName {
			continue
		}
		needAppendComponent = false
		// if old volumeClaimTemplate not support expansion, append it
		if !slices.Contains(x.VolumeClaimTemplateNames, volumeClaimTemplateName) {
			x.VolumeClaimTemplateNames = append(x.VolumeClaimTemplateNames, volumeClaimTemplateName)
			volumeExpandable[i].VolumeClaimTemplateNames = x.VolumeClaimTemplateNames
			needSyncStatusOperations = true
		}
		break
	}
	if needAppendComponent {
		volumeExpandable = append(volumeExpandable, dbaasv1alpha1.OperationComponent{
			Name:                     componentName,
			VolumeClaimTemplateNames: []string{volumeClaimTemplateName},
		})
		needSyncStatusOperations = true
	}
	cluster.Status.Operations.VolumeExpandable = volumeExpandable
	return needSyncStatusOperations
}

// needSyncClusterStatusWhenNotAllowExpansion when StorageClass not allow volume expansion, do it
func needSyncClusterStatusWhenNotAllowExpansion(componentName, volumeClaimTemplateName string, cluster *dbaasv1alpha1.Cluster) bool {
	var (
		needSyncStatusOperations bool
		volumeExpandable         = cluster.Status.Operations.VolumeExpandable
	)
	for i, x := range volumeExpandable {
		if x.Name != componentName {
			continue
		}
		// if old volumeClaimTemplate support expansion, delete it
		index := slices.Index(x.VolumeClaimTemplateNames, volumeClaimTemplateName)
		if index != -1 {
			x.VolumeClaimTemplateNames = append(x.VolumeClaimTemplateNames[:index], x.VolumeClaimTemplateNames[index+1:]...)
			needSyncStatusOperations = true
		}
		// if VolumeClaimTemplateNames is empty, delete the component
		if len(x.VolumeClaimTemplateNames) == 0 {
			volumeExpandable = append(volumeExpandable[:i], volumeExpandable[i+1:]...)
		} else {
			volumeExpandable[i].VolumeClaimTemplateNames = x.VolumeClaimTemplateNames
		}
		break
	}
	cluster.Status.Operations.VolumeExpandable = volumeExpandable
	return needSyncStatusOperations
}

// needSyncClusterStatus check whether sync cluster.status.operations by StorageClass
func needSyncClusterStatus(storageClass *storagev1.StorageClass,
	componentName, volumeClaimTemplateName string,
	cluster *dbaasv1alpha1.Cluster) bool {
	if isSupportVolumeExpansion(storageClass) {
		// if the storageClass support volume expansion
		return needSyncClusterStatusWhenAllowExpansion(componentName, volumeClaimTemplateName, cluster)
	} else {
		return needSyncClusterStatusWhenNotAllowExpansion(componentName, volumeClaimTemplateName, cluster)
	}
}

// handleNoExistsPVC when the cluster not exists PVC, maybe the cluster is creating. so we do the same things with creating a cluster.
func handleNoExistsPVC(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var needSyncStatusOperations bool
	volumeExpandableComponents, err := getSupportVolumeExpansionComponents(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return false, err
	}
	// if volumeExpandable changed, do it
	if !reflect.DeepEqual(cluster.Status.Operations.VolumeExpandable, volumeExpandableComponents) {
		cluster.Status.Operations.VolumeExpandable = volumeExpandableComponents
		needSyncStatusOperations = true
	}
	return needSyncStatusOperations, nil
}
