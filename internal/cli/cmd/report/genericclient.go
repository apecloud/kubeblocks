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

package report

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	kubeblocks "github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type genericClientSet struct {
	client      kubernetes.Interface
	dynamic     dynamic.Interface
	kbClientSet kubeblocks.Interface
}

func NewGenericClientSet(f cmdutil.Factory) (*genericClientSet, error) {
	client, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	dynamic, err := f.DynamicClient()
	if err != nil {
		return nil, err
	}
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	kbClientSet, err := kubeblocks.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &genericClientSet{
		client:      client,
		dynamic:     dynamic,
		kbClientSet: kbClientSet,
	}, nil
}
