/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package multiversion

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
)

// covert appsv1alpha1.servicedescriptor resources to appsv1.servicedescriptor

var (
	sdResource = "servicedescriptors"
	sdGVR      = appsv1.GroupVersion.WithResource(sdResource)
)

func init() {
	hook.RegisterCRDConversion(sdGVR, hook.NewNoVersion(1, 0), &sdConvertor{},
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

type sdConvertor struct{}

func (c *sdConvertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}
