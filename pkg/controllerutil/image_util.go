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
	//  Import these crypto algorithm so that the image parser can work with digest
	_ "crypto/sha256"
	_ "crypto/sha512"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/distribution/reference"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var imageLogger = log.Log.WithName("ImageUtil")

type RegistryConfig struct {
	From                     string `mapstructure:"from"`
	To                       string `mapstructure:"to"`
	RegistryDefaultNamespace string `mapstructure:"registryDefaultNamespace"`

	// Quote from https://docs.docker.com/reference/cli/docker/image/tag/
	// > While the OCI Distribution Specification supports more than two slash-separated components,
	// > most registries only support two slash-separated components.
	// > For Docker's public registry, the path format is as follows:
	// > [NAMESPACE/]REPOSITORY: The first, optional component is typically a user's or an organization's
	// > namespace. The second, mandatory component is the repository name. When the namespace is
	// > not present, Docker uses `library` as the default namespace.
	//
	// So if there are more than two components (like `a/b` as a namespace), specify them both,
	// or they won't be matched. Note empty namespace is legal too.
	//
	// key is the orignal namespace, value is the new namespace
	NamespaceMapping map[string]string `mapstructure:"namespaceMapping"`
}

type RegistriesConfig struct {
	DefaultRegistry  string           `mapstructure:"defaultRegistry"`
	DefaultNamespace string           `mapstructure:"defaultNamespace"`
	RegistryConfig   []RegistryConfig `mapstructure:"registryConfig"`
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

func ReloadRegistryConfig() error {
	registriesConfigMutex.Lock()
	registriesConfig = &RegistriesConfig{}
	if err := viper.UnmarshalKey(constant.CfgRegistries, &registriesConfig); err != nil {
		return err
	}
	registriesConfigMutex.Unlock()

	for _, registry := range registriesConfig.RegistryConfig {
		if len(registry.From) == 0 {
			return errors.New("registries config invalid: from can't be empty")
		}

		if len(registry.To) == 0 {
			return errors.New("registries config invalid: to can't be empty")
		}
	}

	// since the use of kb tools image is widespread, set viper value here so that we don't need
	// to replace it every time
	viper.Set(constant.KBToolsImage, ReplaceImageRegistry(viper.GetString(constant.KBToolsImage)))

	imageLogger.Info("registriesConfig reloaded", "registriesConfig", registriesConfig)
	return nil
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
		imageLogger.Error(err, "parse image failed, the image remains unchanged", "image", image)
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

func ReplaceImageRegistry(image string) string {
	registry, namespace, repository, remainder, err := parseImageName(image)
	// if parse has failed, return the original image. Since k8s will always error an invalid image, we
	// don't need to deal with the error here
	if err != nil {
		return image
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

			for orig, new := range registryMapping.NamespaceMapping {
				if namespace == orig {
					dstNamespace = &new
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
		return fmt.Sprintf("%v/%v%v", dstRegistry, repository, remainder)
	}
	return fmt.Sprintf("%v/%v/%v%v", dstRegistry, *dstNamespace, repository, remainder)
}
