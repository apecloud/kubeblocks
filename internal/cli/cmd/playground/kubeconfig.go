/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package playground

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

// writeKubeConfigOptions provide a set of options for writing a KubeConfig file
type writeKubeConfigOptions struct {
	UpdateExisting       bool
	UpdateCurrentContext bool
	OverwriteExisting    bool
}

// kubeConfigWrite writes the kubeconfig to the specified output
func kubeConfigWrite(kubeConfigStr string, output string, options writeKubeConfigOptions) error {
	var err error

	// convert the kubeconfig string to a kubeconfig object
	kubeConfig, err := clientcmd.Load([]byte(kubeConfigStr))
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// if output is not specified, use the default kubeconfig path
	if output == "" {
		output, err = kubeConfigGetDefaultPath()
		if err != nil {
			return fmt.Errorf("failed to get default kubeconfig path: %w", err)
		}
	}

	// simply write the kubeconfig to the output path, ignoring existing contents
	if options.OverwriteExisting || output == "-" {
		return kubeConfigWriteToPath(kubeConfig, output)
	}

	var existingKubeConfig *clientcmdapi.Config
	firstRun := true
	for {
		existingKubeConfig, err = clientcmd.LoadFromFile(output)
		if err == nil {
			break
		}

		// the output file does not exist, try to create it and try again
		if os.IsNotExist(err) && firstRun {
			klog.V(1).Infof("Output path '%s' does not exist, try to create it", output)

			// create directory path
			if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
				return fmt.Errorf("failed to create output directory '%s': %w", filepath.Dir(output), err)
			}

			// create output file
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("failed to create output file '%s': %w", output, err)
			}
			f.Close()

			// try again
			firstRun = false
			continue
		}
		return fmt.Errorf("failed to load kubeconfig from output path '%s': %w", output, err)
	}
	return kubeConfigMerge(kubeConfig, existingKubeConfig, output, options)
}

// kubeConfigGetDefaultPath returns the path of the default kubeconfig, but errors
// if the KUBECONFIG env var specifies more than one file
func kubeConfigGetDefaultPath() (string, error) {
	defaultKubeConfigLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if len(defaultKubeConfigLoadingRules.GetLoadingPrecedence()) > 1 {
		return "", fmt.Errorf("multiple kubeconfigs specified via KUBECONFIG env var: Please reduce to one entry, unset KUBECONFIG or explicitly choose an output")
	}
	return defaultKubeConfigLoadingRules.GetDefaultFilename(), nil
}

// kubeConfigMerge merges the kubeconfig into the existing kubeconfig and writes it to the output path
func kubeConfigMerge(kubeConfig, existingKubeConfig *clientcmdapi.Config, output string, options writeKubeConfigOptions) error {
	klog.V(1).Infof("Merging new kubeconfig:\n%+v\n>>> into existing Kubeconfig:\n%+v", kubeConfig, existingKubeConfig)

	// overwrite values in existing kubeconfig with values from new kubeconfig
	for k, v := range kubeConfig.Clusters {
		if _, ok := existingKubeConfig.Clusters[k]; ok && !options.UpdateExisting {
			return fmt.Errorf("cluster \"%s\" already exists in target kubeconfig", k)
		}
		existingKubeConfig.Clusters[k] = v
	}

	for k, v := range kubeConfig.AuthInfos {
		if _, ok := existingKubeConfig.AuthInfos[k]; ok && !options.UpdateExisting {
			return fmt.Errorf("user '%s' already exists in target KubeConfig", k)
		}
		existingKubeConfig.AuthInfos[k] = v
	}

	for k, v := range kubeConfig.Contexts {
		if _, ok := existingKubeConfig.Contexts[k]; ok && !options.UpdateExisting {
			return fmt.Errorf("context '%s' already exists in target KubeConfig", k)
		}
		existingKubeConfig.Contexts[k] = v
	}

	// set current context if it is not set, or we want to update it
	if existingKubeConfig.CurrentContext == "" || options.UpdateCurrentContext {
		klog.V(1).Infof("Setting new current-context '%s'", kubeConfig.CurrentContext)
		existingKubeConfig.CurrentContext = kubeConfig.CurrentContext
	}

	return kubeConfigAtomicWrite(existingKubeConfig, output)
}

// kubeConfigWriteToPath takes a kubeconfig and writes it to some path, which can be '-' for os.Stdout
func kubeConfigWriteToPath(kubeconfig *clientcmdapi.Config, path string) error {
	var output *os.File
	defer output.Close()
	var err error

	if path == "-" {
		output = os.Stdout
	} else {
		output, err = os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create file '%s': %w", path, err)
		}
		defer output.Close()
	}

	kubeconfigBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	_, err = output.Write(kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("failed to write file '%s': %w", output.Name(), err)
	}

	klog.V(1).Infof("Wrote kubeconfig to '%s'", output.Name())

	return nil
}

// kubeConfigWrite writes a kubeconfig to a path atomically
func kubeConfigAtomicWrite(kubeconfig *clientcmdapi.Config, path string) error {
	tempPath := fmt.Sprintf("%s.kb_playground_%s", path, time.Now().Format("20060102_150405.000000"))
	if err := clientcmd.WriteToFile(*kubeconfig, tempPath); err != nil {
		return fmt.Errorf("failed to write merged kubeconfig to temporary file '%s': %w", tempPath, err)
	}

	// Move temporary file over existing KubeConfig
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to overwrite existing KubeConfig '%s' with new kubeconfig '%s': %w", path, tempPath, err)
	}

	klog.V(1).Infof("Wrote kubeconfig to '%s'", path)

	return nil
}

// kubeConfigRemove removes the specified kubeconfig from the specified cfgPath path
func kubeConfigRemove(kubeConfigStr string, cfgPath string) error {
	// convert the kubeconfig string to a kubeconfig object
	kubeConfig, err := clientcmd.Load([]byte(kubeConfigStr))
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// get the existing kubeconfig
	existingKubeConfig, err := clientcmd.LoadFromFile(cfgPath)
	if err != nil {
		return err
	}

	for k := range kubeConfig.Clusters {
		delete(existingKubeConfig.Clusters, k)
	}

	for k := range kubeConfig.AuthInfos {
		delete(existingKubeConfig.AuthInfos, k)
	}

	for k := range kubeConfig.Contexts {
		delete(existingKubeConfig.Contexts, k)
	}

	return kubeConfigAtomicWrite(existingKubeConfig, cfgPath)
}

func kubeConfigCurrentContext(kubeConfigStr string) (string, error) {
	kubeConfig, err := clientcmd.Load([]byte(kubeConfigStr))
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	return kubeConfig.CurrentContext, nil
}

func kubeConfigCurrentContextFromFile(kubeConfigPath string) (string, error) {
	kubeConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	return kubeConfig.CurrentContext, nil
}
