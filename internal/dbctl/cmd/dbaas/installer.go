/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"context"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

type Installer struct {
	HelmCfg *action.Configuration

	Namespace string
	Version   string
	Sets      []string
	client    dynamic.Interface
}

func (i *Installer) Install() (string, error) {
	entry := &repo.Entry{
		Name: types.KubeBlocksChartName,
		URL:  types.KubeBlocksChartURL,
	}
	if err := helm.AddRepo(entry); err != nil {
		return "", err
	}

	var sets []string
	for _, set := range i.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Chart:     types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:      true,
		Version:   i.Version,
		Namespace: i.Namespace,
		Sets:      sets,
		Login:     true,
		TryTimes:  2,
	}

	notes, err := chart.Install(i.HelmCfg)
	if err != nil {
		return "", err
	}

	return notes, nil
}

// Uninstall remove dbaas
func (i *Installer) Uninstall() error {
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Namespace: i.Namespace,
	}

	if err := chart.UnInstall(i.HelmCfg); err != nil {
		return err
	}

	// patch clusterdefinition and appversion's finalizer
	ctx := context.Background()
	clusterDefGVR := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceClusterDefinitions}
	cdList, err := i.client.Resource(clusterDefGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, cd := range cdList.Items {
		_, err := i.client.Resource(clusterDefGVR).Patch(ctx, cd.GetName(), k8sapitypes.JSONPatchType, []byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}
	appVerGVR := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceAppVersions}
	appVerList, err := i.client.Resource(appVerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, appVer := range appVerList.Items {
		_, err := i.client.Resource(appVerGVR).Patch(ctx, appVer.GetName(), k8sapitypes.JSONPatchType, []byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
