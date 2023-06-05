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
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/builder"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

var ReplicaSetSignature = func(_ kubekeyapiv1alpha2.Cluster, _ any) {}

func createClusterWithOptions(values *gotemplate.TplValues) (*kubekeyapiv1alpha2.ClusterSpec, error) {
	const tplFile = "kubekey_cluster.tpl"
	rendered, err := builder.BuildFromTemplate(values, tplFile)
	if err != nil {
		return nil, err
	}

	cluster, err := builder.BuildResourceFromYaml(kubekeyapiv1alpha2.Cluster{}, rendered)
	if err != nil {
		return nil, err
	}
	return &cluster.Spec, nil
}

const (
	builtinClusterNameObject    = "Name"
	builtinTimeoutObject        = "Timeout"
	builtinClusterVersionObject = "Version"
	builtinCRITypeObject        = "CRIType"
	builtinUserObject           = "User"
	builtinPasswordObject       = "Password"
	builtinPrivateKeyObject     = "PrivateKey"
	builtinHostsObject          = "Hosts"
	builtinRoleGroupsObject     = "RoleGroups"
)

func buildTemplateParams(o *clusterOptions) *gotemplate.TplValues {
	return &gotemplate.TplValues{
		builtinClusterNameObject:    o.clusterName,
		builtinClusterVersionObject: o.version.KubernetesVersion,
		builtinCRITypeObject:        o.criType,
		builtinUserObject:           o.userName,
		builtinPasswordObject:       o.password,
		builtinPrivateKeyObject:     o.privateKey,
		builtinHostsObject:          o.cluster.Nodes,
		builtinTimeoutObject:        o.timeout,
		builtinRoleGroupsObject: gotemplate.TplValues{
			common.ETCD:   o.cluster.ETCD,
			common.Master: o.cluster.Master,
			common.Worker: o.cluster.Worker,
		},
	}
}
