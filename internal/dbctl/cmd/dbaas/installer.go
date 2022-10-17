/*
Copyright 2022 The KubeBlocks Authors

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
	"encoding/json"

	"helm.sh/helm/v3/pkg/action"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

// Installer will handle the playground cluster creation and management
type Installer struct {
	cfg *action.Configuration

	Namespace string
	Version   string
	Sets      string
	client    dynamic.Interface
}

func (i *Installer) Install() error {
	var sets []string
	if err := json.Unmarshal([]byte(i.Sets), &sets); err != nil {
		return err
	}
	chart := helm.InstallOpts{
		Name:      types.DbaasHelmName,
		Chart:     types.DbaasHelmChart,
		Wait:      true,
		Version:   i.Version,
		Namespace: i.Namespace,
		Sets:      sets,
		Login:     true,
		TryTimes:  2,
	}

	err := chart.Install(i.cfg)
	if err != nil {
		return err
	}

	return nil
}

// Uninstall remove dbaas
func (i *Installer) Uninstall() error {
	chart := helm.InstallOpts{
		Name:      types.DbaasHelmName,
		Namespace: i.Namespace,
	}

	err := chart.UnInstall(i.cfg)
	if err != nil {
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
		_, err := i.client.Resource(clusterDefGVR).Patch(ctx, cd.GetName(), types2.JSONPatchType, []byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{})
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
		_, err := i.client.Resource(appVerGVR).Patch(ctx, appVer.GetName(), types2.JSONPatchType, []byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
