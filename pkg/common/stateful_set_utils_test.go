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

package common

import (
	"testing"

	apps "k8s.io/api/apps/v1"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

func TestGetParentNameAndOrdinal(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if parent, ordinal := intctrlutil.GetParentNameAndOrdinal(pod); parent != set.Name {
		t.Errorf("Extracted the wrong parent name expected %s found %s", set.Name, parent)
	} else if ordinal != 1 {
		t.Errorf("Extracted the wrong ordinal expected %d found %d", 1, ordinal)
	}
	pod.Name = "1-bar"
	if parent, ordinal := intctrlutil.GetParentNameAndOrdinal(pod); parent != "" {
		t.Error("Expected empty string for non-member Pod parent")
	} else if ordinal != -1 {
		t.Error("Expected -1 for non member Pod ordinal")
	}
}

func TestIsMemberOf(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	set2 := testk8s.NewFakeStatefulSet("bar", 3)
	set2.Name = "foo2"
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if !IsMemberOf(set, pod) {
		t.Error("isMemberOf returned false negative")
	}
	if IsMemberOf(set2, pod) {
		t.Error("isMemberOf returned false positive")
	}
}

func TestStatefulSetPodsAreReady(t *testing.T) {
	sts := testk8s.NewFakeStatefulSet("test", 3)
	testk8s.MockStatefulSetReady(sts)
	ready := statefulSetPodsAreReady(sts, *sts.Spec.Replicas)
	if !ready {
		t.Errorf("StatefulSet pods should be ready")
	}
	convertSts := convertToStatefulSet(sts)
	if convertSts == nil {
		t.Errorf("Convert to statefulSet should be succeed")
	}
	convertSts = convertToStatefulSet(&apps.Deployment{})
	if convertSts != nil {
		t.Errorf("Convert to statefulSet should be failed")
	}
	convertSts = convertToStatefulSet(nil)
	if convertSts != nil {
		t.Errorf("Convert to statefulSet should be failed")
	}
}

func TestSStatefulSetOfComponentIsReady(t *testing.T) {
	sts := testk8s.NewFakeStatefulSet("test", 3)
	testk8s.MockStatefulSetReady(sts)
	ready := statefulSetOfComponentIsReady(sts, true, nil)
	if !ready {
		t.Errorf("StatefulSet should be ready")
	}
	ready = statefulSetOfComponentIsReady(sts, false, nil)
	if ready {
		t.Errorf("StatefulSet should not be ready")
	}
}
