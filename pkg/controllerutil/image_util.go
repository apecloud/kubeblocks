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

package controllerutil

import (
	//  Import the crypto sha256 algorithm for the docker image parser to work
	_ "crypto/sha256"
	"sync"

	//  Import the crypto/sha512 algorithm for the docker image parser to work with 384 and 512 sha hashes
	_ "crypto/sha512"
	"fmt"
	"strings"

	"github.com/distribution/reference"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// RegistryNamespaceConfig maps registry namespaces.
//
// Quote from https://docs.docker.com/reference/cli/docker/image/tag/
// > While the OCI Distribution Specification supports more than two slash-separated components,
// > most registries only support two slash-separated components.
// > For Docker's public registry, the path format is as follows:
// > [NAMESPACE/]REPOSITORY: The first, optional component is typically a user's or an organization's
// > namespace. The second, mandatory component is the repository name. When the namespace is
// > not present, Docker uses `library` as the default namespace.
//
// So if there are more than two components, specify them both, or they won't be matched.
type RegistryNamespaceConfig struct {
	From string
	To   string
}

type RegistryConfig struct {
	From                     string
	To                       string
	RegistryDefaultNamespace string
	Namespaces               []RegistryNamespaceConfig
}

type RegistriesConfig struct {
	DefaultRegistry  string
	DefaultNamespace string
	RegistryConfig   []RegistryConfig
}

// this lock protects r/w to this variable itself,
// not the data it points to
var registriesConfigMutex sync.RWMutex
var registriesConfig = &RegistriesConfig{}

func GetRegistriesConfig() *RegistriesConfig {
	registriesConfigMutex.RLock()
	defer registriesConfigMutex.RUnlock()

	// this will return a copy of the pointer
	return registriesConfig
}

func ReloadRegistryConfig() {
	registriesConfigMutex.Lock()
	defer registriesConfigMutex.Unlock()

	registriesConfig = &RegistriesConfig{}
	if err := viper.UnmarshalKey(constant.CfgRegistries, &registriesConfig); err != nil {
		panic(err)
	}

	for _, registry := range registriesConfig.RegistryConfig {
		if len(registry.From) == 0 {
			panic("from can't be empty")
		}

		if len(registry.To) == 0 {
			panic("to can't be empty")
		}
	}

	logger := log.Log
	logger.Info("registriesConfig reloaded", "registriesConfig", registriesConfig)
}

// For a detailed explanation of an image's format, see:
// https://pkg.go.dev/github.com/distribution/reference

// if registry is omitted, the default (docker hub) will be added.
// if namespace is omiited when using docker hub, the default namespace (library) will be added.
func parseImageName(image string) (
	host string, namespace string, repository string, remainder string, err error,
) {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return
	}

	tagged, ok := named.(reference.Tagged)
	if ok {
		remainder += ":" + tagged.Tag()
	}

	digested, ok := named.(reference.Digested)
	if ok {
		remainder += "@" + digested.Digest().String()
	}

	host = reference.Domain(named)

	pathSplit := strings.Split(reference.Path(named), "/")
	if len(pathSplit) > 1 {
		namespace = strings.Join(pathSplit[:len(pathSplit)-1], "/")
	}
	repository = pathSplit[len(pathSplit)-1]

	return
}

func ReplaceImageRegistry(image string) (string, error) {
	registry, namespace, repository, remainder, err := parseImageName(image)
	if err != nil {
		return "", err
	}
	registriesConfigCopy := GetRegistriesConfig()

	chooseRegistry := func() string {
		if registriesConfigCopy.DefaultRegistry != "" {
			return registriesConfigCopy.DefaultRegistry
		} else {
			return registry
		}
	}

	chooseNamespace := func() *string {
		if registriesConfigCopy.DefaultNamespace != "" {
			return &registriesConfigCopy.DefaultNamespace
		} else {
			return &namespace
		}
	}

	var dstRegistry string
	var dstNamespace *string
	for _, registryMapping := range registriesConfigCopy.RegistryConfig {
		if registryMapping.From == registry {
			dstRegistry = registryMapping.To

			for _, namespaceConf := range registryMapping.Namespaces {
				if namespace == namespaceConf.From {
					dstNamespace = &namespaceConf.To
					break
				}
			}

			if dstNamespace == nil {
				if registryMapping.RegistryDefaultNamespace != "" {
					dstNamespace = &registryMapping.RegistryDefaultNamespace
				} else {
					dstNamespace = &namespace
				}
			}

			break
		}
	}

	// no match in registriesConf.Registries
	if dstRegistry == "" {
		dstRegistry = chooseRegistry()
	}

	if dstNamespace == nil {
		dstNamespace = chooseNamespace()
	}

	if *dstNamespace == "" {
		return fmt.Sprintf("%v/%v%v", dstRegistry, repository, remainder), nil
	}
	return fmt.Sprintf("%v/%v/%v%v", dstRegistry, *dstNamespace, repository, remainder), nil
}
