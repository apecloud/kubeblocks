/*
Copyright ApeCloud, Inc.

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

package apps

const (
	maxConcurReconClusterVersionKey = "MAXCONCURRENTRECONCILES_CLUSTERVERSION"
	maxConcurReconClusterDefKey     = "MAXCONCURRENTRECONCILES_CLUSTERDEF"

	// name of our custom finalizer
	dbClusterFinalizerName      = "cluster.kubeblocks.io/finalizer"
	DBClusterFinalizerName      = "cluster.kubeblocks.io/finalizer"
	dbClusterDefFinalizerName   = "clusterdefinition.kubeblocks.io/finalizer"
	clusterVersionFinalizerName = "clusterversion.kubeblocks.io/finalizer"
	opsRequestFinalizerName     = "opsrequest.kubeblocks.io/finalizer"

	// label keys
	clusterDefLabelKey         = "clusterdefinition.kubeblocks.io/name"
	clusterVersionLabelKey     = "clusterversion.kubeblocks.io/name"
	statefulSetPodNameLabelKey = "statefulset.kubernetes.io/pod-name"

	// annotations keys
	lifecycleAnnotationKey = "cluster.kubeblocks.io/lifecycle"
	// debugClusterAnnotationKey is used when one wants to debug the cluster.
	// If debugClusterAnnotationKey = 'on',
	// logs will be recorded in more detail, and some ephemeral pods (esp. those created by jobs) will retain after execution.
	debugClusterAnnotationKey = "cluster.kubeblocks.io/debug"

	// annotations values
	lifecycleDeletePVCAnnotation = "delete-pvc"
)
