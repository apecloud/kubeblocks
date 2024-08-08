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

package component

import (
	"context"
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
)

func buildServiceReferences(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component) error {
	if err := buildServiceReferencesWithoutResolve(ctx, cli, synthesizedComp, compDef, comp); err != nil {
		return err
	}
	return resolveServiceReferences(ctx, cli, synthesizedComp)
}

func buildServiceReferencesWithoutResolve(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component) error {
	if compDef == nil || comp == nil || len(compDef.Spec.ServiceRefDeclarations) == 0 {
		return nil
	}

	serviceRefs := map[string]*appsv1alpha1.ServiceRef{}
	for i, serviceRef := range comp.Spec.ServiceRefs {
		serviceRefs[serviceRef.Name] = &comp.Spec.ServiceRefs[i]
	}

	serviceReferences := make(map[string]*appsv1alpha1.ServiceDescriptor, len(compDef.Spec.ServiceRefDeclarations))
	for _, serviceRefDecl := range compDef.Spec.ServiceRefDeclarations {
		serviceRef, ok := serviceRefs[serviceRefDecl.Name]
		if !ok {
			if IsGenerated(comp) || (serviceRefDecl.Optional != nil && *serviceRefDecl.Optional) {
				continue
			}
			return fmt.Errorf("service-ref for %s is not defined", serviceRefDecl.Name)
		}

		var (
			namespace = synthesizedComp.Namespace
			sd        *appsv1alpha1.ServiceDescriptor
			err       error
		)
		switch {
		case serviceRef.Cluster != "":
			sd, err = handleServiceRefFromCluster(ctx, cli, namespace, *serviceRef, serviceRefDecl, true)
		case serviceRef.ClusterServiceSelector != nil:
			sd, err = handleServiceRefFromCluster(ctx, cli, namespace, *serviceRef, serviceRefDecl, false)
		case serviceRef.ServiceDescriptor != "":
			sd, err = handleServiceRefFromServiceDescriptor(ctx, cli, namespace, *serviceRef, serviceRefDecl)
		}
		if err != nil {
			return err
		}
		serviceReferences[serviceRefDecl.Name] = sd
	}

	if len(serviceReferences) > 0 {
		synthesizedComp.ServiceReferences = serviceReferences
	}
	return nil
}

func handleServiceRefFromCluster(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1alpha1.ServiceRef, serviceRefDecl appsv1alpha1.ServiceRefDeclaration, legacy bool) (*appsv1alpha1.ServiceDescriptor, error) {
	resolver := referencedVars
	if legacy {
		resolver = referencedVars4Legacy
	}
	vars, err := resolver(ctx, cli, namespace, serviceRef)
	if err != nil {
		return nil, err
	}

	// just in-memory service descriptor object, the namespace and name are trivial
	b := builder.NewServiceDescriptorBuilder(namespace, serviceRefDecl.Name).
		SetServiceVersion("").
		SetServiceKind("")
	for i, s := range []func(appsv1alpha1.CredentialVar) *builder.ServiceDescriptorBuilder{b.SetEndpoint, b.SetHost, b.SetPort, b.SetAuthUsername, b.SetAuthPassword} {
		if vars[i] != nil {
			s(*vars[i])
		}
	}
	return b.GetObject(), nil
}

func referencedVars(ctx context.Context, cli client.Reader, namespace string, serviceRef appsv1alpha1.ServiceRef) ([]*appsv1alpha1.CredentialVar, error) {
	var (
		vars = []*appsv1alpha1.CredentialVar{nil, nil, nil, nil, nil}
		err  error
	)
	vars[0], vars[1], vars[2], err = referencedServiceVars(ctx, cli, namespace, serviceRef)
	if err != nil {
		return nil, err
	}
	vars[3], vars[4], err = referencedCredentialVars(ctx, cli, namespace, serviceRef)
	if err != nil {
		return nil, err
	}
	return vars, nil
}

func referencedServiceVars(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1alpha1.ServiceRef) (*appsv1alpha1.CredentialVar, *appsv1alpha1.CredentialVar, *appsv1alpha1.CredentialVar, error) {
	var (
		selector   = serviceRef.ClusterServiceSelector
		host, port *appsv1alpha1.CredentialVar
		obj        any
		err        error
	)

	if selector.Service == nil {
		return nil, nil, nil, nil
	}

	if serviceRef.Namespace != "" {
		namespace = serviceRef.Namespace
	}
	switch {
	case len(selector.Service.Component) == 0:
		obj, err = clusterServiceGetter(ctx, cli, namespace, selector.Cluster, selector.Service.Service)
	case selector.Service.Service == "headless":
		obj, err = headlessCompServiceGetter(ctx, cli, namespace, selector.Cluster, selector.Service.Component)
	default:
		obj, err = compServiceGetter(ctx, cli, namespace, selector.Cluster, selector.Service.Component, selector.Service.Service)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	host = &appsv1alpha1.CredentialVar{Value: composeHostValueFromServices(obj)}
	if p := composePortValueFromServices(obj, selector.Service.Port); p != nil {
		port = &appsv1alpha1.CredentialVar{Value: *p}
	}

	endpoint := func() *appsv1alpha1.CredentialVar {
		hval := host.Value
		if port == nil {
			return &appsv1alpha1.CredentialVar{Value: hval}
		}
		if strings.Contains(hval, ",") {
			// pod-service, the port value has format: host1:port1,host2,port2,...
			return &appsv1alpha1.CredentialVar{Value: port.Value}
		}
		return &appsv1alpha1.CredentialVar{Value: fmt.Sprintf("%s:%s", hval, port.Value)}
	}
	return endpoint(), host, port, nil
}

func referencedCredentialVars(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1alpha1.ServiceRef) (*appsv1alpha1.CredentialVar, *appsv1alpha1.CredentialVar, error) {
	var (
		selector = serviceRef.ClusterServiceSelector
		vars     = []*appsv1alpha1.CredentialVar{nil, nil}
	)

	if selector.Credential == nil {
		return nil, nil, nil
	}

	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      constant.GenerateAccountSecretName(selector.Cluster, selector.Credential.Component, selector.Credential.Name),
	}
	if serviceRef.Namespace != "" {
		secretKey.Namespace = serviceRef.Namespace
	}
	secret := &corev1.Secret{}
	if err := cli.Get(ctx, secretKey, secret); err != nil {
		return nil, nil, err
	}

	for idx, key := range []string{constant.AccountNameForSecret, constant.AccountPasswdForSecret} {
		if _, ok := secret.Data[key]; ok {
			if secret.Namespace == namespace {
				vars[idx] = &appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
							Key:                  key,
						},
					},
				}
			} else {
				vars[idx] = &appsv1alpha1.CredentialVar{
					Value: string(secret.Data[key]),
				}
			}
		}
	}
	return vars[0], vars[1], nil
}

func referencedVars4Legacy(ctx context.Context, cli client.Reader, namespace string, serviceRef appsv1alpha1.ServiceRef) ([]*appsv1alpha1.CredentialVar, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: func() string {
			if serviceRef.Namespace != "" {
				return serviceRef.Namespace
			}
			return namespace
		}(),
		// keep this to reference a legacy cluster
		Name: fmt.Sprintf("%s-conn-credential", serviceRef.Cluster),
	}
	if err := cli.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	vars := []*appsv1alpha1.CredentialVar{nil, nil, nil, nil, nil}
	keys := []string{
		constant.ServiceDescriptorEndpointKey,
		constant.ServiceDescriptorHostKey,
		constant.ServiceDescriptorPortKey,
		constant.ServiceDescriptorUsernameKey,
		constant.ServiceDescriptorPasswordKey,
	}
	for idx, key := range keys {
		if key == constant.ServiceDescriptorHostKey {
			// don't set the host for legacy clusters
			continue
		}
		if _, ok := secret.Data[key]; ok {
			vars[idx] = &appsv1alpha1.CredentialVar{
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
						Key:                  key,
					},
				},
			}
		}
	}
	return vars, nil
}

// handleServiceRefFromServiceDescriptor handles the service reference is provided by external ServiceDescriptor object.
func handleServiceRefFromServiceDescriptor(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1alpha1.ServiceRef, serviceRefDecl appsv1alpha1.ServiceRefDeclaration) (*appsv1alpha1.ServiceDescriptor, error) {
	// verify service kind and version
	verifyServiceKindAndVersion := func(serviceDescriptor appsv1alpha1.ServiceDescriptor, _ ...appsv1alpha1.ServiceRefDeclarationSpec) bool {
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

	if len(serviceRef.Namespace) > 0 {
		namespace = serviceRef.Namespace
	}
	serviceDescriptorKey := client.ObjectKey{
		Namespace: namespace,
		Name:      serviceRef.ServiceDescriptor,
	}
	serviceDescriptor := &appsv1alpha1.ServiceDescriptor{}
	if err := cli.Get(ctx, serviceDescriptorKey, serviceDescriptor); err != nil {
		return nil, err
	}
	if serviceDescriptor.Status.Phase != appsv1alpha1.AvailablePhase {
		return nil, fmt.Errorf("service descriptor %s status is not available", serviceDescriptor.Name)
	}

	match := verifyServiceKindAndVersion(*serviceDescriptor, serviceRefDecl.ServiceRefDeclarationSpecs...)
	if !match {
		return nil, fmt.Errorf("service descriptor %s kind or version is not match with service reference declaration %s", serviceDescriptor.Name, serviceRefDecl.Name)
	}
	return serviceDescriptor, nil
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
