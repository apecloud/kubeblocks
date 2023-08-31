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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func GenServiceReferences(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
) (map[string]*appsv1alpha1.ServiceConnectionCredential, error) {
	if cluster == nil || clusterCompDef == nil || clusterCompSpec == nil {
		return nil, nil
	}

	if len(clusterCompDef.ServiceRefDeclarations) == 0 {
		return nil, nil
	}

	serviceReferences := make(map[string]*appsv1alpha1.ServiceConnectionCredential, len(clusterCompDef.ServiceRefDeclarations))
	for _, serviceRefDecl := range clusterCompDef.ServiceRefDeclarations {
		for _, serviceRef := range clusterCompSpec.ServiceRefs {
			if serviceRef.Name != serviceRefDecl.Name {
				continue
			}

			// if service reference is another KubeBlocks Cluster, then it is necessary to generate a service connection credential from the cluster default connection credential
			if serviceRef.Cluster != "" {
				if serviceRef.Cluster == cluster.Name {
					return nil, fmt.Errorf("cluster %s cannot reference itself", cluster.Name)
				}
				referencedCluster := &appsv1alpha1.Cluster{}
				if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace, Name: serviceRef.Cluster}, referencedCluster); err != nil {
					return nil, err
				}
				secretRefName := component.GenerateConnCredential(referencedCluster.Name)
				secretRef := &corev1.Secret{}
				if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace, Name: secretRefName}, secretRef); err != nil {
					return nil, err
				}
				sccBuilder := builder.NewServiceConnectionCredentialBuilder(reqCtx.Req.Namespace, serviceRefDecl.Name)
				// use cd.Spec.Type as the default Kind and use cluster.Spec.ClusterVersionRef as the default Version
				sccBuilder.SetKind(clusterDef.Spec.Type)
				sccBuilder.SetVersion(cluster.Spec.ClusterVersionRef)
				sccBuilder.SetEndpoint(appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
							Key:                  "endpoint",
						},
					},
				})
				sccBuilder.SetAuth(appsv1alpha1.ConnectionCredentialAuth{
					Username: &appsv1alpha1.CredentialVar{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  "username",
							},
						},
					},
					Password: &appsv1alpha1.CredentialVar{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  "password",
							},
						},
					},
				})
				sccBuilder.SetPort(appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
							Key:                  "port",
						},
					},
				})
				serviceReferences[serviceRefDecl.Name] = sccBuilder.GetObject()
			}

			if serviceRef.ConnectionCredential != "" {
				serviceConnCredential := &appsv1alpha1.ServiceConnectionCredential{}
				if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace, Name: serviceRef.ConnectionCredential}, serviceConnCredential); err != nil {
					return nil, err
				}
				if serviceConnCredential.Spec.Kind != serviceRefDecl.Kind || serviceConnCredential.Spec.Version != serviceRefDecl.Version {
					return nil, fmt.Errorf("service connection credential %s kind or version is not match with service reference declaration %s", serviceConnCredential.Name, serviceRefDecl.Name)
				}
				serviceReferences[serviceRefDecl.Name] = serviceConnCredential
			}
		}
	}
	return serviceReferences, nil
}
