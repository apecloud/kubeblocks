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

package component

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	roclient "github.com/apecloud/kubeblocks/pkg/controller/client"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func GenServiceReferencesLegacy(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (map[string]*appsv1alpha1.ServiceDescriptor, error) {
	var (
		compDef *appsv1alpha1.ComponentDefinition
		comp    *appsv1alpha1.Component
		err     error
	)
	if compDef, err = BuildComponentDefinition(clusterDef, clusterVer, clusterCompSpec); err != nil {
		return nil, err
	}
	if comp, err = BuildComponent(cluster, clusterCompSpec); err != nil {
		return nil, err
	}
	return GenServiceReferences(reqCtx, cli, cluster.Namespace, cluster.Name, compDef, comp)
}

func GenServiceReferences(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
	namespace, clusterName string,
	compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component) (map[string]*appsv1alpha1.ServiceDescriptor, error) {
	if compDef == nil || comp == nil {
		return nil, nil
	}

	if len(compDef.Spec.ServiceRefDeclarations) == 0 {
		return nil, nil
	}

	serviceReferences := make(map[string]*appsv1alpha1.ServiceDescriptor, len(compDef.Spec.ServiceRefDeclarations))
	for _, serviceRefDecl := range compDef.Spec.ServiceRefDeclarations {
		for _, serviceRef := range comp.Spec.ServiceRefs {
			if serviceRef.Name != serviceRefDecl.Name {
				continue
			}
			targetNamespace := namespace
			if serviceRef.Namespace != "" {
				targetNamespace = serviceRef.Namespace
			}
			// if service reference is another KubeBlocks Cluster, then it is necessary to generate a service connection credential from the cluster connection credential secret
			if serviceRef.Cluster != "" {
				if err := handleClusterTypeServiceRef(reqCtx, cli, targetNamespace, clusterName, serviceRef, serviceRefDecl, serviceReferences); err != nil {
					return nil, err
				}
				// serviceRef.Cluster takes precedence, and if serviceRef.Cluster is set, serviceRef.ServiceDescriptor will be ignored
				break
			}

			if serviceRef.ServiceDescriptor != "" {
				if err := handleServiceDescriptorTypeServiceRef(reqCtx, cli, targetNamespace, serviceRef, serviceRefDecl, serviceReferences); err != nil {
					return nil, err
				}
			}
		}
		// _, exist := serviceReferences[serviceRefDecl.Name]
		// if !exist {
		//	 return nil, fmt.Errorf("componentDef %s's serviceRefDeclaration %s has not been defined, please check if there is corresponding service definition and binding in Cluster.spec.componentSpecs[*].serviceRefs", clusterCompDef.Name, serviceRefDecl.Name)
		// }
	}
	if len(serviceReferences) == 0 {
		return nil, nil
	}
	return serviceReferences, nil
}

// handleClusterTypeServiceRef handles the service reference is another KubeBlocks Cluster.
func handleClusterTypeServiceRef(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
	namespace, clusterName string,
	serviceRef appsv1alpha1.ServiceRef,
	serviceRefDecl appsv1alpha1.ServiceRefDeclaration,
	serviceReferences map[string]*appsv1alpha1.ServiceDescriptor) error {
	if serviceRef.Cluster == clusterName {
		return fmt.Errorf("cluster %s cannot reference itself", clusterName)
	}
	referencedCluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: namespace, Name: serviceRef.Cluster}, referencedCluster); err != nil {
		return err
	}

	// get the connection credential secret of the referenced cluster
	secretRef := &corev1.Secret{}
	secretRefName := constant.GenerateDefaultConnCredential(referencedCluster.Name)
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: namespace, Name: secretRefName}, secretRef); err != nil {
		return err
	}

	handleSecretKey := func(key string, setter func(appsv1alpha1.CredentialVar) *builder.ServiceDescriptorBuilder) {
		if _, ok := secretRef.Data[key]; ok {
			setter(appsv1alpha1.CredentialVar{
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
						Key:                  key,
					},
				},
			})
		}
	}

	// TODO: Second-stage optimization: Cluster-type references no longer perform conversion on the connection credential field. Instead, the configMap or secret is directly passed through to the serviceDescriptor.
	sdBuilder := builder.NewServiceDescriptorBuilder(namespace, generateDefaultServiceDescriptorName(clusterName))
	sdBuilder.SetServiceKind("")
	sdBuilder.SetServiceVersion("")
	handleSecretKey(constant.ServiceDescriptorEndpointKey, sdBuilder.SetEndpoint)
	handleSecretKey(constant.ServiceDescriptorPortKey, sdBuilder.SetPort)
	handleSecretKey(constant.ServiceDescriptorUsernameKey, sdBuilder.SetAuthUsername)
	handleSecretKey(constant.ServiceDescriptorPasswordKey, sdBuilder.SetAuthPassword)
	serviceReferences[serviceRefDecl.Name] = sdBuilder.GetObject()
	return nil
}

// handleServiceDescriptorTypeServiceRef handles the service reference is provided by external ServiceDescriptor object.
func handleServiceDescriptorTypeServiceRef(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
	namespace string,
	serviceRef appsv1alpha1.ServiceRef,
	serviceRefDecl appsv1alpha1.ServiceRefDeclaration,
	serviceReferences map[string]*appsv1alpha1.ServiceDescriptor) error {
	// verify service kind and version
	verifyServiceKindAndVersion := func(serviceDescriptor appsv1alpha1.ServiceDescriptor, serviceRefDeclSpecs ...appsv1alpha1.ServiceRefDeclarationSpec) bool {
		for _, serviceRefDeclSpec := range serviceRefDecl.ServiceRefDeclarationSpecs {
			if getWellKnownServiceKindAliasMapping(serviceRefDeclSpec.ServiceKind) != getWellKnownServiceKindAliasMapping(serviceDescriptor.Spec.ServiceKind) {
				continue
			}
			versionMatch := verifyServiceVersion(serviceDescriptor.Spec.ServiceVersion, serviceRefDeclSpec.ServiceVersion)
			if versionMatch {
				return true
			}
		}
		return false
	}
	serviceDescriptor := &appsv1alpha1.ServiceDescriptor{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: namespace, Name: serviceRef.ServiceDescriptor}, serviceDescriptor); err != nil {
		return err
	}
	if serviceDescriptor.Status.Phase != appsv1alpha1.AvailablePhase {
		return fmt.Errorf("service descriptor %s status is not available", serviceDescriptor.Name)
	}
	match := verifyServiceKindAndVersion(*serviceDescriptor, serviceRefDecl.ServiceRefDeclarationSpecs...)
	if !match {
		return fmt.Errorf("service descriptor %s kind or version is not match with service reference declaration %s", serviceDescriptor.Name, serviceRefDecl.Name)
	}
	serviceReferences[serviceRefDecl.Name] = serviceDescriptor
	return nil
}

func verifyServiceVersion(serviceDescriptorVersion, serviceRefDeclarationServiceVersion string) bool {
	isRegex := false
	regex, err := regexp.Compile(serviceRefDeclarationServiceVersion)
	if err == nil {
		isRegex = true
	}
	if !isRegex {
		return serviceDescriptorVersion == serviceRefDeclarationServiceVersion
	}
	return regex.MatchString(serviceDescriptorVersion)
}

func getWellKnownServiceKindAliasMapping(serviceKind string) string {
	lowerServiceKind := strings.ToLower(serviceKind)
	switch {
	case slices.Contains(constant.GetZookeeperAlias(), lowerServiceKind):
		return constant.ServiceKindZookeeper
	case slices.Contains(constant.GetElasticSearchAlias(), lowerServiceKind):
		return constant.ServiceKindElasticSearch
	case slices.Contains(constant.GetMongoDBAlias(), lowerServiceKind):
		return constant.ServiceKindMongoDB
	case slices.Contains(constant.GetPostgreSQLAlias(), lowerServiceKind):
		return constant.ServiceKindPostgreSQL
	case slices.Contains(constant.GetClickHouseAlias(), lowerServiceKind):
		return constant.ServiceKindClickHouse
	default:
		return lowerServiceKind
	}
}

func generateDefaultServiceDescriptorName(clusterName string) string {
	return fmt.Sprintf("kbsd-%s", constant.GenerateDefaultConnCredential(clusterName))
}
