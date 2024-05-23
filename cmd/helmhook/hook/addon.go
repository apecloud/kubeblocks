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

package hook

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	errors "sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type Addon struct {
	BasedHandler

	KeepAddons bool
}

const (
	helmResourcePolicyKey  = "helm.sh/resource-policy"
	helmResourcePolicyKeep = "keep"
)

func (p *Addon) IsSkip(*UpgradeContext) (bool, error) {
	return !p.KeepAddons, nil
}

func (p *Addon) Handle(ctx *UpgradeContext) (err error) {
	addons, err := ctx.KBClient.ExtensionsV1alpha1().Addons().List(ctx, metav1.ListOptions{
		LabelSelector: toLabelSelector(addonSelectorLabels()),
	})

	if err != nil {
		return errors.IgnoreNotFound(err)
	}

	for _, addon := range addons.Items {
		if addon.GetDeletionTimestamp() != nil {
			continue
		}
		// Allow unused addons to be updated or deleted
		if addon.Spec.InstallSpec == nil || !addon.Spec.InstallSpec.Enabled {
			Log("addon[%s] is not installed and pass", addon.GetName())
			continue
		}
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return patchDisableUpdateAddon(ctx, ctx.KBClient, addon)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func patchDisableUpdateAddon(ctx context.Context, client *versioned.Clientset, addon extensionsv1alpha1.Addon) error {
	annotations := addon.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if annotations[helmResourcePolicyKey] == helmResourcePolicyKeep {
		return nil
	}
	annotations[helmResourcePolicyKey] = helmResourcePolicyKeep
	patchBytes, _ := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{"annotations": annotations}})

	_, err := client.ExtensionsV1alpha1().Addons().Patch(ctx, addon.Name, types.MergePatchType,
		patchBytes, metav1.PatchOptions{})
	return err
}
