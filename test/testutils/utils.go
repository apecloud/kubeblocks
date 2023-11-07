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

package testutils

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	K8sCoreAPIVersion  = "v1"
	ResourceConfigmaps = "configmaps"
)

// Extensions API group
const (
	ExtensionsAPIGroup   = "extensions.kubeblocks.io"
	ExtensionsAPIVersion = "v1alpha1"
	ResourceAddons       = "addons"
)

// The purpose of this package is to remove the dependency of the KubeBlocks project on 'pkg/cli'.
var (
	// KubeBlocksRepoName helm repo name for kubeblocks
	KubeBlocksRepoName = "kubeblocks"

	// KubeBlocksChartName helm chart name for kubeblocks
	KubeBlocksChartName = "kubeblocks"

	// KubeBlocksReleaseName helm release name for kubeblocks
	KubeBlocksReleaseName = "kubeblocks"

	// KubeBlocksChartURL the helm chart repo for installing kubeblocks
	KubeBlocksChartURL = "https://apecloud.github.io/helm-charts"

	// GitLabHelmChartRepo the helm chart repo in GitLab
	GitLabHelmChartRepo = "https://jihulab.com/api/v4/projects/85949/packages/helm/stable"

	// CfgKeyHelmRepoURL the helm repo url key
	CfgKeyHelmRepoURL = "HELM_REPO_URL"
)

func ConfigmapGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: ResourceConfigmaps}
}

func AddonGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: ExtensionsAPIGroup, Version: ExtensionsAPIVersion, Resource: ResourceAddons}
}

var addToScheme sync.Once

func NewFactory() cmdutil.Factory {
	configFlags := NewConfigFlagNoWarnings()
	// Add CRDs to the scheme. They are missing by default.
	addToScheme.Do(func() {
		if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
			// This should never happen.
			panic(err)
		}
	})
	return cmdutil.NewFactory(configFlags)
}

// NewConfigFlagNoWarnings returns a ConfigFlags that disables warnings.
func NewConfigFlagNoWarnings() *genericclioptions.ConfigFlags {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.WrapConfigFn = func(c *rest.Config) *rest.Config {
		c.WarningHandler = rest.NoWarnings{}
		return c
	}
	return configFlags
}

// GetResourceObjectFromGVR queries the resource object using GVR.
func GetResourceObjectFromGVR(gvr schema.GroupVersionResource, key client.ObjectKey, client dynamic.Interface, k8sObj interface{}) error {
	unstructuredObj, err := client.
		Resource(gvr).
		Namespace(key.Namespace).
		Get(context.TODO(), key.Name, metav1.GetOptions{})
	if err != nil {
		return core.WrapError(err, "failed to get resource[%v]", key)
	}
	return apiruntime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, k8sObj)
}

// GetHelmChartRepoURL gets helm chart repo, chooses one from GitHub and GitLab based on the IP location
func GetHelmChartRepoURL() string {

	// if helm repo url is specified by config or environment, use it
	url := viper.GetString(CfgKeyHelmRepoURL)
	if url != "" {
		klog.V(1).Infof("Using helm repo url set by config or environment: %s", url)
		return url
	}

	// if helm repo url is not specified, choose one from GitHub and GitLab based on the IP location
	// if location is CN, or we can not get location, use GitLab helm chart repo
	repo := KubeBlocksChartURL
	location, _ := GetIPLocation()
	if location == "CN" || location == "" {
		repo = GitLabHelmChartRepo
	}
	klog.V(1).Infof("Using helm repo url: %s", repo)
	return repo
}

func GetIPLocation() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://ifconfig.io/country_code", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	location, err := io.ReadAll(resp.Body)
	if len(location) == 0 || err != nil {
		return "", err
	}

	// remove last "\n"
	return string(location[:len(location)-1]), nil
}
