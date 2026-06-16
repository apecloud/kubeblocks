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

package util

import "testing"

func TestEnvListMapConversion(t *testing.T) {
	got := EnvL2M([]string{"A=1", "B=two=parts", "EMPTY"})
	if got["A"] != "1" || got["B"] != "two=parts" || got["EMPTY"] != "" {
		t.Fatalf("unexpected env map: %#v", got)
	}

	roundTrip := EnvL2M(EnvM2L(map[string]string{"A": "1", "B": ""}))
	if roundTrip["A"] != "1" || roundTrip["B"] != "" {
		t.Fatalf("unexpected round trip map: %#v", roundTrip)
	}
}

func TestDefaultEnvVarsAndAccessors(t *testing.T) {
	envVars := DefaultEnvVars()
	if len(envVars) != 4 {
		t.Fatalf("DefaultEnvVars() length = %d, want 4", len(envVars))
	}
	fields := map[string]string{}
	for _, env := range envVars {
		if env.ValueFrom == nil || env.ValueFrom.FieldRef == nil {
			t.Fatalf("env %s should use fieldRef", env.Name)
		}
		fields[env.Name] = env.ValueFrom.FieldRef.FieldPath
	}
	if fields[kbEnvNamespace] != "metadata.namespace" ||
		fields[kbEnvPodName] != "metadata.name" ||
		fields[kbEnvPodUID] != "metadata.uid" ||
		fields[kbEnvNodeName] != "spec.nodeName" {
		t.Fatalf("unexpected field refs: %#v", fields)
	}

	t.Setenv(kbEnvNamespace, "ns")
	t.Setenv(kbEnvPodName, "pod")
	t.Setenv(kbEnvPodUID, "uid")
	t.Setenv(kbEnvNodeName, "node")
	if namespace() != "ns" || PodName() != "pod" || podName() != "pod" || podUID() != "uid" || nodeName() != "node" {
		t.Fatalf("unexpected env accessors: namespace=%q pod=%q uid=%q node=%q", namespace(), podName(), podUID(), nodeName())
	}
}
