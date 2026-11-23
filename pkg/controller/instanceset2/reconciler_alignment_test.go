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

package instanceset2

import (
	"reflect"
	"testing"

	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
)

func TestAlignmentBuildsInstancesWithEffectiveConfigs(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline named", true: "compressed flat"}[compressed], func(t *testing.T) {
			its, tree, member := newITS2ConfigFixture(t, compressed)
			source := its.DeepCopy()
			if _, err := NewValidationReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if _, err := NewAlignmentReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			instances := its2ConfigInstances(tree)
			if len(instances) != 2 {
				t.Fatalf("generated instances = %v, want two", instances)
			}
			assertITS2ConfigIntent(t, instances[member], "main-a", "specific")
			assertITS2ConfigIntent(t, instances["demo-0"], "main", "global")
			if !reflect.DeepEqual(its.Spec, source.Spec) {
				t.Fatal("building child Instances changed source InstanceSet spec")
			}

			// Replicas share a resolved template, but each child owns its config association.
			templates, err := instancetemplate.BuildInstanceTemplateExtMap(its, tree)
			if err != nil {
				t.Fatal(err)
			}
			template := templates[member]
			before := *template.Configs[0].DeepCopy()
			for range 2 {
				child, err := buildInstanceByTemplate(tree, member, template, its)
				if err != nil {
					t.Fatal(err)
				}
				assertITS2ConfigIntent(t, child, "main-a", "specific")
				child.Spec.Configs[0].Volume.ConfigMapName = "child-only"
			}
			if !reflect.DeepEqual(template.Configs[0], before) || template.Configs[0].Volume == nil {
				t.Fatal("building child Instance modified the resolved template config")
			}
		})
	}
}
