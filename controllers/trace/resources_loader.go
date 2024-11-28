/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package trace

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type resourcesLoader struct{}

func (r *resourcesLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	// load trace object
	tree, err := kubebuilderx.ReadObjectTree[*tracev1.ReconciliationTrace](ctx, reader, req, nil)
	if err != nil {
		return nil, err
	}
	if tree.GetRoot() == nil {
		return tree, nil
	}

	// load i18n resources
	i18n := &corev1.ConfigMap{}
	i18nResourcesNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	i18nResourcesName := viper.GetString(constant.I18nResourcesName)
	if err = reader.Get(ctx, types.NamespacedName{Namespace: i18nResourcesNamespace, Name: i18nResourcesName}, i18n); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if err = tree.Add(i18n); err != nil {
		return nil, err
	}

	tree.EventRecorder = recorder
	tree.Logger = logger
	tree.SetFinalizer(finalizer)

	return tree, nil
}

func traceResources() kubebuilderx.TreeLoader {
	return &resourcesLoader{}
}

var _ kubebuilderx.TreeLoader = &resourcesLoader{}
