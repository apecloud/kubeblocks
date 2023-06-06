/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
