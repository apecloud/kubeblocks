/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package plan

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestRestorePVCComponentUIDOnlyOnNewRestore(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "cluster-db", Namespace: "default", UID: "component-uid"}}
	template := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: comp.Namespace},
		Spec: dpv1alpha1.RestoreSpec{PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
			RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{Replicas: 2,
				Templates: []dpv1alpha1.RestoreVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db"}}}},
		}},
	}
	for _, existing := range []bool{false, true} {
		name := "new"
		var objects []client.Object
		if existing {
			name = "existing"
			objects = append(objects, template.DeepCopy())
		}
		t.Run(name, func(t *testing.T) {
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			manager := NewRestoreManager(context.Background(), cli, &appsv1.Cluster{}, scheme, nil, 1, 0)
			if err := manager.createRestoreAndWait(comp, template.DeepCopy()); err == nil {
				t.Fatal("unfinished Restore must keep waiting")
			}
			fetched := &dpv1alpha1.Restore{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(template), fetched); err != nil {
				t.Fatal(err)
			}
			claim := fetched.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.Templates[0]
			expected := string(comp.UID)
			if existing {
				expected = ""
			}
			if claim.Annotations[constant.RestorePVCComponentUIDAnnotationKey] != expected {
				t.Fatalf("marker=%q want %q", claim.Annotations[constant.RestorePVCComponentUIDAnnotationKey], expected)
			}
			if !existing && !metav1.IsControlledBy(fetched, comp) {
				t.Fatal("new Restore must retain Component owner")
			}
			if len(claim.OwnerReferences) != 0 {
				t.Fatal("claim controller must remain available for InstanceSet adoption")
			}
		})
	}
}
