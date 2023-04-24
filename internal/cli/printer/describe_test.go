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
