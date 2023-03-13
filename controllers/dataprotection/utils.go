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

package dataprotection

import (
	corev1 "k8s.io/api/core/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

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

// byPodName sorts a list of jobs by pod name
type byPodName []corev1.Pod

// Len return the length of byBackupStartTime, for the sort.Sort
func (c byPodName) Len() int {
	return len(c)
}

// Swap the items, for the sort.Sort
func (c byPodName) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// Less define how to compare items, for the sort.Sort
func (c byPodName) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}
