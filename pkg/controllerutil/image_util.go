package controllerutil

import (
	//  Import the crypto sha256 algorithm for the docker image parser to work
	_ "crypto/sha256"
	"fmt"
	"strings"

	//  Import the crypto/sha512 algorithm for the docker image parser to work with 384 and 512 sha hashes
	_ "crypto/sha512"

	"github.com/distribution/reference"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// Quote from https://docs.docker.com/reference/cli/docker/image/tag/
// > While the OCI Distribution Specification supports more than two slash-separated components,
// > most registries only support two slash-separated components.
// > For Docker's public registry, the path format is as follows:
// > [NAMESPACE/]REPOSITORY: The first, optional component is typically a user's or an organization's
// namespace. The second, mandatory component is the repository name. When the namespace is
// not present, Docker uses `library` as the default namespace.
//
// So if there are more than two components, specify them both, or they won't be matched.
type registryNamespaceConfig struct {
	From string
	To   string
}

type registryConfig struct {
	From       string
	To         string
	Namespaces []registryNamespaceConfig
}

type registriesConfig struct {
	DefaultRegistry  string
	DefaultNamespace string
	Registries       []registryConfig
}

var registriesConf = registriesConfig{}

func init() {
	viper.UnmarshalKey(constant.CfgRegistries, &registriesConf)

	// TODO: validate
	for _, registry := range registriesConf.Registries {
		if len(registry.From) == 0 {
			panic("from can't be empty")
		}

		for _, namespace := range registry.Namespaces {
			if len(namespace.From) == 0 || len(namespace.To) == 0 {
				panic("namespace can't be empty")
			}
		}
	}
}

// For a detailed explaination of an image's format, see:
// https://kubernetes.io/docs/concepts/containers/images/

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

	chooseRegistry := func() string {
		if registriesConf.DefaultRegistry != "" {
			return registriesConf.DefaultRegistry
		} else {
			return registry
		}
	}

	chooseNamespace := func() string {
		if registriesConf.DefaultNamespace != "" {
			return registriesConf.DefaultNamespace
		} else {
			return namespace
		}
	}

	var dstRegistry, dstNamespace string
	for _, registryMapping := range registriesConf.Registries {
		if registryMapping.From == registry {
			if len(registryMapping.To) != 0 {
				dstRegistry = registryMapping.To
			} else {
				dstRegistry = chooseRegistry()
			}

			for _, namespaceConf := range registryMapping.Namespaces {
				if namespace == namespaceConf.From {
					dstNamespace = namespaceConf.To
					break
				}
			}

			if dstNamespace == "" {
				dstNamespace = namespace
			}

			break
		}
	}

	// no match in registriesConf.Registries
	if dstRegistry == "" {
		dstRegistry = chooseRegistry()
		dstNamespace = chooseNamespace()
	}

	return fmt.Sprintf("%v/%v/%v%v", dstRegistry, dstNamespace, repository, remainder), nil
}
