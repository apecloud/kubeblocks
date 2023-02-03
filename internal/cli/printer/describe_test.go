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

package printer

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
)

func TestPrintAllWarningEvents(t *testing.T) {
	eventList := &corev1.EventList{}
	eventList.Items = []corev1.Event{{
		Type:    corev1.EventTypeNormal,
		Reason:  "EventSucceed",
		Message: "event succeed",
		TypeMeta: metav1.TypeMeta{
			Kind: "Event",
		}}}
	out := &bytes.Buffer{}
	PrintAllWarningEvents(eventList, out)
	assert.Equal(t, "\nWarning Events: "+NoneString+"\n", out.String())

	reason, message := "EventFailed", "event failed"
	name := "pod-test-xkdsl1"
	eventList.Items = append(eventList.Items, corev1.Event{
		Type:    corev1.EventTypeWarning,
		Reason:  reason,
		Message: message,
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: name,
		},
	})
	PrintAllWarningEvents(eventList, out)
	if !clitesting.ContainExpectStrings(out.String(), "TIME", "TYPE", "REASON", "OBJECT", "MESSAGE") {
		t.Fatal(`Expect warning events output: "TIME	TYPE	REASON	OBJECT	MESSAGE"`)
	}
	object := "Instance/" + name
	if !clitesting.ContainExpectStrings(out.String(), corev1.EventTypeWarning, reason, message, object) {
		t.Fatalf(`Expect warning events output: "%s	%s	%s	%s"`,
			corev1.EventTypeWarning, reason, message, object)
	}
}

func TestPrintConditions(t *testing.T) {
	conditionType, reason := "Created", "CreateResources"
	message := "Failed to create resources"
	conditions := []metav1.Condition{
		{
			Type:    "Initialize",
			Reason:  "InitResources",
			Status:  "True",
			Message: "Start to init resources",
		},
		{
			Type:    conditionType,
			Reason:  reason,
			Status:  metav1.ConditionFalse,
			Message: message,
		},
	}
	out := &bytes.Buffer{}
	PrintConditions(conditions, out)
	if !clitesting.ContainExpectStrings(out.String(), "LAST-TRANSITION-TIME", "TYPE", "REASON", "STATUS", "MESSAGE") {
		t.Fatal(`Expect conditions output: "LAST-TRANSITION-TIME	TYPE	REASON	STATUS	MESSAGE"`)
	}
	if !clitesting.ContainExpectStrings(out.String(), conditionType, reason, string(metav1.ConditionFalse), message) {
		t.Fatalf(`Expect conditions output: "%s	%s	%s	%s"`, conditionType, reason, metav1.ConditionFalse, message)
	}
}
