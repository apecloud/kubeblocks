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

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
)

// covert extensionsv1alpha1.addon resources to extensionsv1.addon

var (
	addonResource = "addons"
	// addonGVR      = extensionsv1.GroupVersion.WithResource(addonResource)
	addonGVR = extensionsv1alpha1.GroupVersion.WithResource(addonResource)
)

func init() {
	hook.RegisterCRDConversion(addonGVR, hook.NewNoVersion(1, 0), &addonConvertor{},
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

type addonConvertor struct{}

func (c *addonConvertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}
