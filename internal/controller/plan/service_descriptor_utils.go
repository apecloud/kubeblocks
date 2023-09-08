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

package plan

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func GenServiceReferences(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
) (map[string]*appsv1alpha1.ServiceDescriptor, error) {
	if cluster == nil || clusterCompDef == nil || clusterCompSpec == nil {
		return nil, nil
	}

	if len(clusterCompDef.ServiceRefDeclarations) == 0 {
		return nil, nil
	}

	serviceReferences := make(map[string]*appsv1alpha1.ServiceDescriptor, len(clusterCompDef.ServiceRefDeclarations))
	for _, serviceRefDecl := range clusterCompDef.ServiceRefDeclarations {
		for _, serviceRef := range clusterCompSpec.ServiceRefs {
			if serviceRef.Name != serviceRefDecl.Name {
				continue
			}
			targetNamespace := cluster.Namespace
			if serviceRef.Namespace != "" {
				targetNamespace = serviceRef.Namespace
			}
			// if service reference is another KubeBlocks Cluster, then it is necessary to generate a service connection credential from the cluster connection credential secret
			if serviceRef.Cluster != "" {
				if serviceRef.Cluster == cluster.Name {
					return nil, fmt.Errorf("cluster %s cannot reference itself", cluster.Name)
				}
				referencedCluster := &appsv1alpha1.Cluster{}
				if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: targetNamespace, Name: serviceRef.Cluster}, referencedCluster); err != nil {
					return nil, err
				}
				referencedClusterDef := &appsv1alpha1.ClusterDefinition{}
				if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: referencedCluster.Spec.ClusterDefRef}, referencedClusterDef); err != nil {
					return nil, err
				}

				// get the connection credential secret of the referenced cluster
				var sdName string
				secretRef := &corev1.Secret{}
				if serviceRef.ConnectionCredential != "" {
					sdName = component.GenerateCustomServiceDescriptorName(serviceRef.ConnectionCredential)
					if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: targetNamespace, Name: serviceRef.ConnectionCredential}, secretRef); err != nil {
						return nil, err
					}
				} else {
					sdName = component.GenerateDefaultServiceDescriptorName(cluster.Name)
					secretRefName := component.GenerateConnCredential(referencedCluster.Name)
					if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: targetNamespace, Name: secretRefName}, secretRef); err != nil {
						return nil, err
					}
				}

				sdBuilder := builder.NewServiceDescriptorBuilder(targetNamespace, sdName)
				// use cd.Spec.Type as the default Kind and use cluster.Spec.ClusterVersionRef as the default Version
				sdBuilder.SetServiceKind(referencedClusterDef.Spec.Type)
				sdBuilder.SetServiceVersion(referencedCluster.Spec.ClusterVersionRef)
				sdBuilder.SetEndpoint(appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
							Key:                  constant.ServiceDescriptorEndpointKey,
						},
					},
				})
				sdBuilder.SetAuth(appsv1alpha1.ConnectionCredentialAuth{
					Username: &appsv1alpha1.CredentialVar{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  constant.ServiceDescriptorUsernameKey,
							},
						},
					},
					Password: &appsv1alpha1.CredentialVar{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  constant.ServiceDescriptorPasswordKey,
							},
						},
					},
				})
				sdBuilder.SetPort(appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
							Key:                  constant.ServiceDescriptorPortKey,
						},
					},
				})
				serviceReferences[serviceRefDecl.Name] = sdBuilder.GetObject()
				// serviceRef.Cluster takes precedence, and if serviceRef.Cluster is set, serviceRef.ServiceDescriptor will be ignored
				break
			}

			if serviceRef.ServiceDescriptor != "" {
				// verify service kind and version
				verifyServiceKindAndVersion := func(serviceDescriptor appsv1alpha1.ServiceDescriptor, serviceRefDeclSpecs ...appsv1alpha1.ServiceRefDeclarationSpec) bool {
					for _, serviceRefDeclSpec := range serviceRefDecl.ServiceRefDeclarationSpecs {
						if serviceRefDeclSpec.ServiceKind != serviceDescriptor.Spec.ServiceKind {
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
				if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: targetNamespace, Name: serviceRef.ServiceDescriptor}, serviceDescriptor); err != nil {
					return nil, err
				}
				if serviceDescriptor.Status.Phase != appsv1alpha1.AvailablePhase {
					return nil, fmt.Errorf("service descriptor %s status is not available", serviceDescriptor.Name)
				}
				match := verifyServiceKindAndVersion(*serviceDescriptor, serviceRefDecl.ServiceRefDeclarationSpecs...)
				if !match {
					return nil, fmt.Errorf("service descriptor %s kind or version is not match with service reference declaration %s", serviceDescriptor.Name, serviceRefDecl.Name)
				}
				serviceReferences[serviceRefDecl.Name] = serviceDescriptor
			}
		}
		_, exist := serviceReferences[serviceRefDecl.Name]
		if !exist {
			return nil, fmt.Errorf("componentDef %s's serviceRefDeclaration %s has not been defined, please check if there is corresponding service definition and binding in Cluster.spec.componentSpecs[*].serviceRefs", clusterCompDef.Name, serviceRefDecl.Name)
		}
	}
	return serviceReferences, nil
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
