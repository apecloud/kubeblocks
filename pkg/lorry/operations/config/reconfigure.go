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

package config

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/lorry/util/kubernetes"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type Reconfigure struct {
	operations.Base
	client        *rest.RESTClient
	clusterName   string
	componentName string
	namespace     string
}

var reconfigure operations.Operation = &Reconfigure{}

func init() {
	err := operations.Register(string(util.ReconfigureOperation), reconfigure)
	if err != nil {
		panic(err.Error())
	}
}

func (r *Reconfigure) Init(context.Context) error {
	client, err := kubernetes.GetRESTClientForKB()
	if err != nil {
		return errors.New("get rest client for KB failed")
	}

	r.client = client
	r.clusterName = viper.GetString(constant.KBEnvClusterName)
	r.componentName = viper.GetString(constant.KBEnvCompName)
	r.namespace = viper.GetString(constant.KBEnvNamespace)
	return nil
}

func (r *Reconfigure) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	configFileName := req.GetString("configFileName")
	configSpecName := req.GetString("configSpecName")
	if configFileName == "" || configSpecName == "" {
		return nil, errors.New("configFileName and configSpecName must be set")
	}

	parameter, ok := req.Parameters["parameter"].(map[string]string)
	if !ok || len(parameter) == 0 {
		return nil, errors.New("reconfigure parameters must be set to map[string]string")
	}

	var keys []v1alpha1.ParameterPair
	for k, v := range parameter {
		keys = append(keys, v1alpha1.ParameterPair{
			Key:   k,
			Value: pointer.String(v),
		})
	}

	reconfigureReq := &v1alpha1.OpsRequest{
		Spec: v1alpha1.OpsRequestSpec{
			Type:       v1alpha1.ReconfiguringType,
			ClusterRef: r.clusterName,
			Reconfigure: &v1alpha1.Reconfigure{
				ComponentOps: v1alpha1.ComponentOps{
					ComponentName: r.componentName,
				},
				Configurations: []v1alpha1.ConfigurationItem{{
					Name: configSpecName,
					Keys: []v1alpha1.ParameterConfig{{
						Key:        configFileName,
						Parameters: keys,
					}},
				}},
			},
		},
	}

	ops := &v1alpha1.OpsRequest{}
	return nil, r.client.Post().
		Namespace(r.namespace).
		Resource(v1alpha1.OpsRequestKind).
		Name(fmt.Sprintf("%s-%s-", r.clusterName, string(v1alpha1.ReconfiguringType))).
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
		Body(reconfigureReq).
		Do(ctx).
		Into(ops)
}
