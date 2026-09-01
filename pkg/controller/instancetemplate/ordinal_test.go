/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instancetemplate

import "testing"

func TestGetTemplateNameAndOrdinal(t *testing.T) {
	testCases := []struct {
		name         string
		workloadName string
		podName      string
		templateName string
		ordinal      int32
		wantErr      bool
	}{
		{name: "default template", workloadName: "cluster-comp", podName: "cluster-comp-3", ordinal: 3},
		{name: "template with dashes", workloadName: "cluster-comp", podName: "cluster-comp-b-a-3", templateName: "b-a", ordinal: 3},
		{name: "workload suffix is not a template", workloadName: "cluster-comp-a", podName: "cluster-comp-a-3", ordinal: 3},
		{name: "different workload", workloadName: "cluster-comp", podName: "other-comp-3", wantErr: true},
		{name: "missing ordinal", workloadName: "cluster-comp", podName: "cluster-comp-template-", wantErr: true},
		{name: "invalid ordinal", workloadName: "cluster-comp", podName: "cluster-comp-template-x", wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			templateName, ordinal, err := GetTemplateNameAndOrdinal(testCase.workloadName, testCase.podName)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetTemplateNameAndOrdinal() error = %v", err)
			}
			if templateName != testCase.templateName || ordinal != testCase.ordinal {
				t.Fatalf("GetTemplateNameAndOrdinal() = %q, %d; want %q, %d", templateName, ordinal, testCase.templateName, testCase.ordinal)
			}
		})
	}
}
