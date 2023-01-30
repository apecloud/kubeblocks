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

package dataprotection

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

func checkResourceExists(
	ctx context.Context,
	client client.Client,
	key client.ObjectKey,
	obj client.Object) (bool, error) {

	if err := client.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// if not found, returns not exists and nil error.
			return false, nil
		}
		return false, err
	}
	// if found, return true
	return true, nil
}

// byBackupStartTime sorts a list of jobs by start timestamp, using their names as a tie breaker.
type byBackupStartTime []dataprotectionv1alpha1.Backup

// Len return the length of byBackupStartTime, for the sort.Sort
func (o byBackupStartTime) Len() int { return len(o) }

// Swap the items, for the sort.Sort
func (o byBackupStartTime) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

// Less define how to compare items, for the sort.Sort
func (o byBackupStartTime) Less(i, j int) bool {
	if o[i].Status.StartTimestamp == nil && o[j].Status.StartTimestamp != nil {
		return false
	}
	if o[i].Status.StartTimestamp != nil && o[j].Status.StartTimestamp == nil {
		return true
	}
	if o[i].Status.StartTimestamp.Equal(o[j].Status.StartTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].Status.StartTimestamp.Before(o[j].Status.StartTimestamp)
}

// DeleteObjectBackground delete the object in the background, usually used in the Reconcile method
func DeleteObjectBackground(cli client.Client, ctx context.Context, obj client.Object) error {
	deletePropagation := metav1.DeletePropagationBackground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &deletePropagation,
	}

	if err := cli.Delete(ctx, obj, deleteOptions); err != nil {
		// failed to delete backups, return error info.
		return err
	}
	return nil
}
