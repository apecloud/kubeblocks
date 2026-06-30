/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"reflect"
	"strings"
	"sync"

	"github.com/distribution/reference"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var imageLogger = log.Log.WithName("ImageUtil")

type registryConfig struct {
	From             string `mapstructure:"from"`
	To               string `mapstructure:"to"`
	DefaultNamespace string `mapstructure:"defaultNamespace"`

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

type registriesConfig struct {
	DefaultRegistry  string           `mapstructure:"defaultRegistry"`
	DefaultNamespace string           `mapstructure:"defaultNamespace"`
	RegistryConfig   []registryConfig `mapstructure:"registryConfig"`
}

// this lock protects r/w to this variable itself,
// not the data it points to
var registriesConfigMutex sync.RWMutex
var registriesConfigInstance = &registriesConfig{}

func getRegistriesConfig() *registriesConfig {
	registriesConfigMutex.RLock()
	defer registriesConfigMutex.RUnlock()

	// this will return a copy of the pointer
	return registriesConfigInstance
}

func LoadRegistryConfig() error {
	newRegistriesConfig := &registriesConfig{}
	if err := viper.UnmarshalKey(constant.CfgRegistries, &newRegistriesConfig); err != nil {
		return err
	}

	for _, registry := range registriesConfigInstance.RegistryConfig {
		if len(registry.From) == 0 {
			return errors.New("registries config invalid: from can't be empty")
		}

		if len(registry.To) == 0 {
			return errors.New("registries config invalid: to can't be empty")
		}
	}

	registriesConfigMutex.Lock()
	registriesConfigInstance = newRegistriesConfig
	registriesConfigMutex.Unlock()

	// since the use of kb tools image is widespread, set viper value here so that we don't need
	// to replace it every time
	viper.Set(constant.KBToolsImage, ReplaceImageRegistry(viper.GetString(constant.KBToolsImage)))

	imageLogger.Info("registriesConfig reloaded", "registriesConfig", registriesConfigInstance)
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
	registriesConfigCopy := getRegistriesConfig()

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
				if registryMapping.DefaultNamespace != "" {
					dstNamespace = &registryMapping.DefaultNamespace
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

// MatchContainerImageInStatus returns true if the status image matches the image
// requested in PodSpec. For digest-pinned images, the status imageID digest is
// the stable runtime contract; status image may be reported as a tag or local ID.
func MatchContainerImageInStatus(specImage, statusImage, statusImageID string) bool {
	specName, specTag, specDigest := splitContainerImageRef(specImage)
	if specDigest != "" {
		_, _, statusDigest := splitContainerImageRef(statusImageID)
		return specDigest == statusDigest
	}

	statusName, statusTag, _ := splitContainerImageRef(statusImage)
	if specTag != "" && specTag != statusTag {
		return false
	}
	return imageBaseName(specName) == imageBaseName(statusName)
}

// EqualContainerImageInSpec returns true if two PodSpec image references point to
// the same image after ignoring registry rewrites. Tags and digests remain strict.
func EqualContainerImageInSpec(oldImage, newImage string) bool {
	if oldImage == newImage {
		return true
	}

	oldName, oldTag, oldDigest := splitContainerImageRef(oldImage)
	newName, newTag, newDigest := splitContainerImageRef(newImage)
	if (oldDigest != "" || newDigest != "") && oldDigest != newDigest {
		return false
	}
	if (oldTag != "" || newTag != "") && oldTag != newTag {
		return false
	}
	return imageBaseName(oldName) == imageBaseName(newName)
}

// splitContainerImageRef separates the name, tag, and digest parts from an
// image reference. It is deliberately lenient because Kubernetes may surface
// runtime-formatted status image strings that are not normalized references.
func splitContainerImageRef(imageName string) (name string, tag string, digest string) {
	searchName := imageName
	slashIndex := strings.Index(imageName, "/")
	if slashIndex > 0 {
		searchName = imageName[slashIndex:]
	} else {
		slashIndex = 0
	}

	id := strings.Index(searchName, "@")
	ic := strings.Index(searchName, ":")
	if ic < 0 && id < 0 {
		return imageName, "", ""
	}
	if id >= 0 && (id < ic || ic < 0) {
		id += slashIndex
		name = imageName[:id]
		digest = strings.TrimPrefix(imageName[id:], "@")
		return name, "", digest
	}
	if id >= 0 && ic >= 0 {
		id += slashIndex
		ic += slashIndex
		name = imageName[:ic]
		tag = strings.TrimPrefix(imageName[ic:id], ":")
		digest = strings.TrimPrefix(imageName[id:], "@")
		return name, tag, digest
	}

	ic += slashIndex
	name = imageName[:ic]
	tag = strings.TrimPrefix(imageName[ic:], ":")
	return name, tag, ""
}

func imageBaseName(name string) string {
	index := strings.LastIndex(name, "/")
	if index < 0 {
		return name
	}
	return name[index+1:]
}

const (
	legacyConfigManagerContainerName = "config-manager"
	legacyConfigManagerToolsInitName = "install-config-manager-tool"
)

// OnlyKBManagedContainerImageChanged checks only the image fields in the given
// container lists. It returns changed=true when at least one known KB-managed
// container image changed, and ok=false when any list shape, non-image field, or
// non-KB-managed image change is detected.
func OnlyKBManagedContainerImageChanged(oldContainers, newContainers []corev1.Container) (changed bool, ok bool) {
	toolsImage := viper.GetString(constant.KBToolsImage)
	if toolsImage == "" {
		return false, false
	}
	return onlyKBManagedContainerImageChanged(oldContainers, newContainers, toolsImage)
}

func onlyKBManagedContainerImageChanged(oldContainers, newContainers []corev1.Container, toolsImage string) (bool, bool) {
	if len(oldContainers) != len(newContainers) {
		return false, false
	}
	changed := false
	for i := range oldContainers {
		oldContainer := oldContainers[i]
		newContainer := newContainers[i]
		if oldContainer.Name != newContainer.Name {
			return false, false
		}
		oldImage := oldContainer.Image
		newImage := newContainer.Image
		oldContainer.Image = ""
		newContainer.Image = ""
		if !reflect.DeepEqual(oldContainer, newContainer) {
			return false, false
		}
		if oldImage == newImage {
			continue
		}
		if !isKBManagedContainerName(oldContainer.Name) ||
			!sameImageRepositoryName(oldImage, toolsImage) ||
			!sameImageRepositoryName(newImage, toolsImage) {
			return false, false
		}
		changed = true
	}
	return changed, true
}

func isKBManagedContainerName(name string) bool {
	switch name {
	case kbagent.ContainerName, kbagent.ContainerName4Worker, kbagent.InitContainerName,
		legacyConfigManagerContainerName, legacyConfigManagerToolsInitName:
		return true
	default:
		return false
	}
}

func sameImageRepositoryName(image, target string) bool {
	imageNamespace, imageRepository, ok := imageRepositoryName(image)
	if !ok {
		return false
	}
	targetNamespace, targetRepository, ok := imageRepositoryName(target)
	if !ok {
		return false
	}
	return imageNamespace == targetNamespace && imageRepository == targetRepository
}

func imageRepositoryName(image string) (string, string, bool) {
	_, namespace, repository, _, err := parseImageName(image)
	if err != nil {
		return "", "", false
	}
	return namespace, repository, true
}
