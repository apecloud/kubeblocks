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

package v1alpha1

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionEnableLogsValidated = "ValidateEnableLogs"
)

// SetStatusCondition classified by Condition.Type
func (c *Cluster) SetStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&c.Status.Conditions, condition)
}

// NewValidateConfigFailedCondition config validate fail in cluster API
func NewValidateConfigFailedCondition(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionEnableLogsValidated,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            message,
	}
}
