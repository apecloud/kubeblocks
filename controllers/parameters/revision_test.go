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

package parameters

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func getCurrentRevision(annotations map[string]string) string {
	if len(annotations) == 0 {
		return ""
	}
	return annotations[constant.ConfigurationRevision]
}

func getLastRevision(annotations map[string]string, revision int64) (configurationRevision, bool) {
	revisions := retrieveRevision(annotations)
	for i := len(revisions) - 1; i >= 0; i-- {
		if revisions[i].revision == revision {
			return revisions[i], true
		}
	}
	return configurationRevision{}, false
}

func TestGcConfigRevision(t *testing.T) {
	cm := builder.NewConfigMapBuilder("default", "test").
		AddAnnotations(core.GenerateRevisionPhaseKey("1"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("2"), "Init").
		AddAnnotations(core.GenerateRevisionPhaseKey("3"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("4"), "Finished").
		GetObject()
	revisions := gcRevision(cm.GetAnnotations())
	assert.Equal(t, 0, len(revisions))

	cm = builder.NewConfigMapBuilder("default", "test").
		AddAnnotations(core.GenerateRevisionPhaseKey("1"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("2"), "Init").
		AddAnnotations(core.GenerateRevisionPhaseKey("3"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("4"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("5"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("6"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("7"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("8"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("9"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("10"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("11"), "Finished").
		AddAnnotations(core.GenerateRevisionPhaseKey("12"), `{"Phase":"Finished","Revision":"12","Policy":"","ExecResult":"","SucceedCount":0,"ExpectedCount":0,"Retry":false,"Failed":false,"Message":"the configuration file has not been modified, skip reconfigure"}`).
		GetObject()

	assert.Equal(t, 12, len(retrieveRevision(cm.GetAnnotations())))

	revisions = gcRevision(cm.GetAnnotations())
	assert.Equal(t, 2, len(revisions))
	assert.Equal(t, string(appsv1alpha1.CInitPhase), string(revisions[1].phase))
	assert.Equal(t, string(appsv1alpha1.CFinishedPhase), string(revisions[0].phase))

	gcConfigRevision(cm)
	assert.Equal(t, 10, len(retrieveRevision(cm.GetAnnotations())))
}

func TestParseRevision(t *testing.T) {
	type args struct {
		revision string
		phase    string
	}
	tests := []struct {
		name    string
		args    args
		want    configurationRevision
		wantErr bool
	}{{
		name: "test",
		args: args{
			revision: "12absdl",
			phase:    "Init",
		},
		want:    configurationRevision{},
		wantErr: true,
	}, {
		name: "test",
		args: args{
			revision: "120000",
			phase:    "Pending",
		},
		want: configurationRevision{
			revision:    120000,
			strRevision: "120000",
			phase:       parametersv1alpha1.CPendingPhase,
			result: parameters.Result{
				Phase:    parametersv1alpha1.CPendingPhase,
				Revision: "120000",
			},
		},
		wantErr: false,
	}, {
		name: "test",
		args: args{
			revision: "",
			phase:    "Init",
		},
		want:    configurationRevision{},
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
			assert.Equalf(t, tt.want, getCurrentRevision(tt.args.annotations), "getCurrentRevision(%v)", tt.args.annotations)
		})
	}
}

func TestGetLastRevision(t *testing.T) {
	type args struct {
		annotations map[string]string
		revision    int64
	}
	tests := []struct {
		name  string
		args  args
		want  configurationRevision
		want1 bool
	}{{
		name: "test",
		args: args{
			annotations: map[string]string{
				core.GenerateRevisionPhaseKey("1"): "Finished",
				core.GenerateRevisionPhaseKey("2"): "Running",
			},
			revision: 2,
		},
		want: configurationRevision{
			revision:    2,
			strRevision: "2",
			phase:       parametersv1alpha1.CRunningPhase,
			result: parameters.Result{
				Phase:    parametersv1alpha1.CRunningPhase,
				Revision: "2",
			},
		},
		want1: true,
	}, {
		name: "test",
		args: args{
			annotations: map[string]string{
				core.GenerateRevisionPhaseKey("1"): "Finished",
				core.GenerateRevisionPhaseKey("2"): "Running",
			},
			revision: 3,
		},
		want1: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := getLastRevision(tt.args.annotations, tt.args.revision)
			assert.Equalf(t, tt.want, got, "getLastRevision(%v, %v)", tt.args.annotations, tt.args.revision)
			assert.Equalf(t, tt.want1, got1, "getLastRevision(%v, %v)", tt.args.annotations, tt.args.revision)
		})
	}
}
