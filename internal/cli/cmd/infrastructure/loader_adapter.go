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
	"bufio"
	"embed"
	"encoding/json"
	"strings"

	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/leaanthony/debme"
	"k8s.io/apimachinery/pkg/util/yaml"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

func newKubeKeyClusterTemplate(templateName string) (string, error) {
	tmplFs, _ := debme.FS(cueTemplate, "template")
	if tmlBytes, err := tmplFs.ReadFile(templateName); err != nil {
		return "", err
	} else {
		return string(tmlBytes), nil
	}
}

func createClusterWithOptions(values *gotemplate.TplValues) (*kubekeyapiv1alpha2.ClusterSpec, error) {
	const tplFile = "kubekey_cluster_template.tpl"
	yamlTemplate, err := newKubeKeyClusterTemplate(tplFile)
	if err != nil {
		return nil, err
	}

	tpl := gotemplate.NewTplEngine(values, nil, "ClusterTemplate", nil, nil)
	rendered, err := tpl.Render(yamlTemplate)
	if err != nil {
		return nil, err
	}

	var ret map[string]interface{}
	cluster := kubekeyapiv1alpha2.Cluster{}
	content, err := yaml.NewYAMLReader(bufio.NewReader(strings.NewReader(rendered))).Read()
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to read the cluster yaml")
	}
	err = yaml.Unmarshal(content, &ret)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to unmarshal the cluster yaml")
	}

	contentToJSON, err := yaml.ToJSON(content)
	if err != nil {
		return nil, cfgcore.WrapError(err, "Unable to convert configuration to json")
	}
	if err := json.Unmarshal(contentToJSON, &cluster); err != nil {
		return nil, cfgcore.WrapError(err, "failed to unmarshal the cluster")
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
		builtinClusterVersionObject: o.version,
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
