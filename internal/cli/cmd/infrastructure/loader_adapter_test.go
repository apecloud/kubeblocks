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

package infrastructure

import (
	"testing"

	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"

	"github.com/apecloud/kubeblocks/internal/configuration/container"
)

func TestCreateClusterWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    *clusterOptions
		want    *kubekeyapiv1alpha2.ClusterSpec
		wantErr bool
	}{{
		name: "generateClusterTest",
		args: &clusterOptions{
			clusterName: "for_test",
			version:     defaultVersion,
			criType:     string(container.ContainerdType),
			userName:    "test",
			password:    "test",
			cluster: Cluster{
				Nodes: []ClusterNode{
					{
						Name:            "node1",
						Address:         "127.0.0.1",
						InternalAddress: "127.0.0.1",
					}, {
						Name:            "node2",
						Address:         "127.0.0.2",
						InternalAddress: "127.0.0.2",
					},
				},
				ETCD:   []string{"node1"},
				Master: []string{"node1"},
				Worker: []string{"node2"},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createClusterWithOptions(buildTemplateParams(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("createClusterWithOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil {
				t.Errorf("createClusterWithOptions() got = %v, want %v", got, tt.want)
			}
		})
	}
}
