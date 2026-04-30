/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEventsToString_Empty(t *testing.T) {
	events := &corev1.EventList{}
	result := EventsToString(events)
	assert.NotNil(t, result)
}

func TestEventsToString_WithEvents(t *testing.T) {
	events := &corev1.EventList{
		Items: []corev1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "ev-1"},
				Type:       "Warning",
				Reason:     "BackupFailed",
				Message:    "backup timed out",
			},
		},
	}
	result := EventsToString(events)
	assert.Contains(t, result, "BackupFailed")
	assert.Contains(t, result, "backup timed out")
}
