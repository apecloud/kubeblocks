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

package controllerutil

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// GetUncachedObjects returns a list of K8s objects, for these object types,
// and their list types, client.Reader will read directly from the API server instead
// of the cache, which may not be up-to-date.
// see sigs.k8s.io/controller-runtime/pkg/client/split.go to understand how client
// works with this UncachedObjects filter.
func GetUncachedObjects() []client.Object {
	// client-side read cache reduces the number of requests processed in the API server,
	// which is good for performance. However, it can sometimes lead to obscure issues,
	// most notably lacking read-after-write consistency, i.e. reading a value immediately
	// after updating it may miss to see the changes.
	// while in most cases this problem can be mitigated by retrying later in an idempotent
	// manner, there are some cases where it cannot, for example if a decision is to be made
	// that has side-effect operations such as returning an error message to the user
	// (in webhook) or deleting certain resources (in controllerutil.HandleCRDeletion).
	// additionally, retry loops cause unnecessary delays when reconciliations are processed.
	// for the sake of performance, now only the objects created by the end-user is listed here,
	// to solve the two problems mentioned above.
	// consider carefully before adding new objects to this list.
	return []client.Object{
		// avoid to cache potential large data objects
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&appsv1alpha1.Cluster{},
	}
}
