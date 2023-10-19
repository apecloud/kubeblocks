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

package tasks

import (
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/apecloud/kubeblocks/pkg/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

func kubeconfigMerge(newKubeConfig *clientcmdapi.Config, existingKubeConfig *clientcmdapi.Config, outPath string) error {
	// merge clusters
	for k, v := range newKubeConfig.Clusters {
		if _, ok := existingKubeConfig.Clusters[k]; ok {
			return cfgcore.MakeError("Cluster '%s' already exists in target KubeConfig", k)
		}
		existingKubeConfig.Clusters[k] = v
	}

	// merge auth
	for k, v := range newKubeConfig.AuthInfos {
		if _, ok := existingKubeConfig.AuthInfos[k]; ok {
			return cfgcore.MakeError("AuthInfo '%s' already exists in target KubeConfig", k)
		}
		existingKubeConfig.AuthInfos[k] = v
	}

	// merge contexts
	for k, v := range newKubeConfig.Contexts {
		if _, ok := existingKubeConfig.Contexts[k]; ok {
			return cfgcore.MakeError("Context '%s' already exists in target KubeConfig", k)
		}
		existingKubeConfig.Contexts[k] = v
	}

	existingKubeConfig.CurrentContext = newKubeConfig.CurrentContext
	return kubeconfigWrite(existingKubeConfig, outPath)
}

func kubeconfigWrite(config *clientcmdapi.Config, path string) error {
	tempPath := fmt.Sprintf("%s.kb_%s", path, time.Now().Format("20060102_150405.000000"))
	if err := clientcmd.WriteToFile(*config, tempPath); err != nil {
		return cfgcore.WrapError(err, "failed to write merged kubeconfig to temporary file '%s'", tempPath)
	}

	// Move temporary file over existing KubeConfig
	if err := os.Rename(tempPath, path); err != nil {
		return cfgcore.WrapError(err, "failed to overwrite existing KubeConfig '%s' with new kubeconfig '%s'", path, tempPath)
	}
	return nil
}

func GetDefaultConfig() string {
	defaultKubeConfigLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kcFile := defaultKubeConfigLoadingRules.GetDefaultFilename()
	if kcFile == "" {
		kcFile = util.GetKubeconfigDir()
	}
	return kcFile
}

func kubeconfigLoad(kcFile string) (*clientcmdapi.Config, error) {
	if _, err := os.Stat(kcFile); err == nil {
		return clientcmd.LoadFromFile(kcFile)
	}
	return nil, nil
}
