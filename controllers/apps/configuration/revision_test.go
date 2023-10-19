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

package configuration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func TestGcConfigRevision(t *testing.T) {
	cm := builder.NewConfigMapBuilder("default", "test").
		AddAnnotations(core.GenerateRevisionPhaseKey("1"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("2"), "init").
		AddAnnotations(core.GenerateRevisionPhaseKey("3"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("4"), "finished").
		GetObject()
	revisions := GcRevision(cm.GetAnnotations())
	assert.Equal(t, 0, len(revisions))

	cm = builder.NewConfigMapBuilder("default", "test").
		AddAnnotations(core.GenerateRevisionPhaseKey("1"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("2"), "init").
		AddAnnotations(core.GenerateRevisionPhaseKey("3"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("4"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("5"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("6"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("7"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("8"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("9"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("10"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("11"), "finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("12"), "finished").
		GetObject()

	assert.Equal(t, 12, len(RetrieveRevision(cm.GetAnnotations())))

	revisions = GcRevision(cm.GetAnnotations())
	assert.Equal(t, 2, len(revisions))
	assert.Equal(t, "init", string(revisions[1].Phase))
	assert.Equal(t, "finished", string(revisions[0].Phase))

	GcConfigRevision(cm)
	assert.Equal(t, 10, len(RetrieveRevision(cm.GetAnnotations())))
}

func TestParseRevision(t *testing.T) {
	type args struct {
		revision string
		phase    string
	}
	tests := []struct {
		name    string
		args    args
		want    ConfigurationRevision
		wantErr bool
	}{{
		name: "test",
		args: args{
			revision: "12absdl",
			phase:    "Init",
		},
		want:    ConfigurationRevision{},
		wantErr: true,
	}, {
		name: "test",
		args: args{
			revision: "120000",
			phase:    "Pending",
		},
		want: ConfigurationRevision{
			StrRevision: "120000",
			Revision:    120000,
			Phase:       appsv1alpha1.CPendingPhase,
		},
		wantErr: false,
	}, {
		name: "test",
		args: args{
			revision: "",
			phase:    "Init",
		},
		want:    ConfigurationRevision{},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRevision(tt.args.revision, tt.args.phase)
			if (err != nil) != tt.wantErr {
				assert.Error(t, err, fmt.Sprintf("parseRevision(%v, %v)", tt.args.revision, tt.args.phase))
			} else {
				assert.Equalf(t, tt.want, got, "parseRevision(%v, %v)", tt.args.revision, tt.args.phase)
			}
		})
	}
}

func TestGetCurrentRevision(t *testing.T) {
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "test",
		args: args{
			annotations: map[string]string{},
		},
		want: "",
	}, {
		name: "test",
		args: args{
			annotations: map[string]string{"abcd": "finished"},
		},
		want: "",
	}, {
		name: "test",
		args: args{
			annotations: map[string]string{constant.ConfigurationRevision: "mytest"},
		},
		want: "mytest",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, GetCurrentRevision(tt.args.annotations), "GetCurrentRevision(%v)", tt.args.annotations)
		})
	}
}
