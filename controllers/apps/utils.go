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

package apps

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

// default reconcile requeue after duration
var requeueDuration = time.Millisecond * 1000

func boolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func mergeMap(dst, src map[string]string) {
	for key, val := range src {
		dst[key] = val
	}
}

func placement(obj client.Object) string {
	if obj == nil || obj.GetAnnotations() == nil {
		return ""
	}
	return obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey]
}

func intoContext(ctx context.Context, placement string) context.Context {
	return multicluster.IntoContext(ctx, placement)
}

func inDataContext4C() *multicluster.ClientOption {
	return multicluster.InDataContext()
}

func inDataContextWithMultiCheck4C() *multicluster.ClientOption {
	return multicluster.InDataContextWithMultiCheck()
}

func inUniversalContext4C() *multicluster.ClientOption {
	return multicluster.InUniversalContext()
}

func inDataContext4G() model.GraphOption {
	return model.WithClientOption(multicluster.InDataContext())
}

func inUniversalContext4G() model.GraphOption {
	return model.WithClientOption(multicluster.InUniversalContext())
}

// inDataContextWithMultiCheck4G If a resource is a mirror object which is created in all data clusters,
// this option needs to be used for GET interface. And if any data cluster does not exist the object, an error will be reported.
func inDataContextWithMultiCheck4G() model.GraphOption {
	return model.WithClientOption(multicluster.InDataContextWithMultiCheck())
}

func clientOption(v *model.ObjectVertex) *multicluster.ClientOption {
	if v.ClientOpt != nil {
		opt, ok := v.ClientOpt.(*multicluster.ClientOption)
		if ok {
			return opt
		}
		panic(fmt.Sprintf("unknown client option: %T", v.ClientOpt))
	}
	return multicluster.InControlContext()
}
