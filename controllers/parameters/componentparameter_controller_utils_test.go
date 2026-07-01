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

package parameters

import (
	"testing"

	"k8s.io/utils/ptr"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	parampkg "github.com/apecloud/kubeblocks/pkg/parameters"
)

func TestNormalizeManagedParameterInputs(t *testing.T) {
	t.Run("updates override assignments and remove is explicit", func(t *testing.T) {
		inputs := &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{
				"max_connections": ptr.To("1000"),
				"sync_binlog":     ptr.To("1"),
			},
			Updates: []parametersv1alpha1.ParameterUpdate{
				{Type: parametersv1alpha1.ParameterUpdateSet, Key: "max_connections", Value: ptr.To("2000")},
				{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "sync_binlog"},
			},
		}

		got, err := normalizeManagedParameterInputs(inputs)
		if err != nil {
			t.Fatalf("normalizeManagedParameterInputs() error = %v", err)
		}
		if got["max_connections"] == nil || *got["max_connections"] != "2000" {
			t.Fatalf("expected max_connections to be overridden to 2000, got %#v", got["max_connections"])
		}
		if _, ok := got["sync_binlog"]; !ok {
			t.Fatalf("expected sync_binlog remove marker to be preserved")
		}
		if got["sync_binlog"] != nil {
			t.Fatalf("expected sync_binlog to normalize to nil remove marker, got %#v", got["sync_binlog"])
		}
	})

	t.Run("set without value is rejected", func(t *testing.T) {
		_, err := normalizeManagedParameterInputs(&parametersv1alpha1.ParameterInputs{
			Updates: []parametersv1alpha1.ParameterUpdate{{
				Type: parametersv1alpha1.ParameterUpdateSet,
				Key:  "max_connections",
			}},
		})
		if err == nil {
			t.Fatalf("expected error for set update without value")
		}
	})
}

func TestMergeItemParameters(t *testing.T) {
	t.Run("override replaces managed parameter overlay for a file", func(t *testing.T) {
		item := &parametersv1alpha1.ConfigTemplateItemDetail{
			ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
				"my.cnf": {
					Content: ptr.To("[mysqld]\nmax_connections=1000\n"),
					Parameters: map[string]*string{
						"max_connections": ptr.To("1000"),
						"sync_binlog":     ptr.To("1"),
					},
				},
			},
		}
		updated := map[string]parametersv1alpha1.ParametersInFile{
			"my.cnf": {
				Parameters: map[string]*string{
					"max_connections": nil,
				},
			},
		}

		mergeItemParameters(item, updated, true)

		got := item.ConfigFileParams["my.cnf"]
		if got.Content == nil || *got.Content != "[mysqld]\nmax_connections=1000\n" {
			t.Fatalf("expected non-managed fields to be preserved, got %#v", got.Content)
		}
		if len(got.Parameters) != 1 {
			t.Fatalf("expected managed overlay to be replaced, got %#v", got.Parameters)
		}
		decoded := parampkg.DecodeParameterOverlay(got.Parameters)
		if _, ok := decoded["max_connections"]; !ok {
			t.Fatalf("expected max_connections remove marker to be kept")
		}
		if decoded["max_connections"] != nil {
			t.Fatalf("expected max_connections to be overridden to nil remove marker, got %#v", decoded["max_connections"])
		}
	})
}

func TestMergeMissingConfigFileParams(t *testing.T) {
	dest := &parametersv1alpha1.ConfigTemplateItemDetail{
		ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
			"my.cnf": {
				Parameters: map[string]*string{
					"max_connections": ptr.To("2000"),
				},
			},
		},
	}
	expected := &parametersv1alpha1.ConfigTemplateItemDetail{
		ConfigFileParams: map[string]parametersv1alpha1.ParametersInFile{
			"my.cnf": {
				Parameters: map[string]*string{
					"max_connections": ptr.To("1000"),
				},
			},
			"log.conf": {
				Parameters: map[string]*string{
					"slow_query_log": ptr.To("1"),
				},
			},
		},
	}

	mergeMissingConfigFileParams(dest, expected)

	if got := dest.ConfigFileParams["my.cnf"].Parameters["max_connections"]; got == nil || *got != "2000" {
		t.Fatalf("expected existing file params to be preserved, got %#v", got)
	}
	if _, ok := dest.ConfigFileParams["log.conf"]; !ok {
		t.Fatalf("expected missing legacy file params to be merged")
	}

	*expected.ConfigFileParams["log.conf"].Parameters["slow_query_log"] = "0"
	if got := dest.ConfigFileParams["log.conf"].Parameters["slow_query_log"]; got == nil || *got != "1" {
		t.Fatalf("expected merged params to be deep-copied, got %#v", got)
	}
}

func TestNilEmptyConfigItemDetailsEquivalence(t *testing.T) {
	t.Run("nil and empty ConfigItemDetails should be treated as equal", func(t *testing.T) {
		merged := parampkg.MergeComponentParameter(
			&parametersv1alpha1.ComponentParameter{},
			&parametersv1alpha1.ComponentParameter{},
			func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {},
		)
		var nilDetails []parametersv1alpha1.ConfigTemplateItemDetail
		emptyDetails := merged.Spec.ConfigItemDetails
		if len(nilDetails) != 0 || len(emptyDetails) != 0 {
			t.Fatalf("expected both to be empty, got nil=%d empty=%d", len(nilDetails), len(emptyDetails))
		}
		bothEmpty := len(nilDetails) == 0 && len(emptyDetails) == 0
		if !bothEmpty {
			t.Fatalf("nil and empty ConfigItemDetails should be treated as equal")
		}
	})
}
