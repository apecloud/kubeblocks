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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func buildServiceReferences(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, compDef *appsv1.ComponentDefinition, comp *appsv1.Component) error {
	if err := buildServiceReferencesWithoutResolve(ctx, cli, synthesizedComp, compDef, comp); err != nil {
		return err
	}
	return resolveServiceReferences(ctx, cli, synthesizedComp)
}

func buildServiceReferencesWithoutResolve(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, compDef *appsv1.ComponentDefinition, comp *appsv1.Component) error {
	if compDef == nil || comp == nil || len(compDef.Spec.ServiceRefDeclarations) == 0 {
		return nil
	}

	serviceRefs := map[string]*appsv1.ServiceRef{}
	for i, serviceRef := range comp.Spec.ServiceRefs {
		serviceRefs[serviceRef.Name] = &comp.Spec.ServiceRefs[i]
	}

	serviceReferences := make(map[string]*appsv1.ServiceDescriptor, len(compDef.Spec.ServiceRefDeclarations))
	for _, serviceRefDecl := range compDef.Spec.ServiceRefDeclarations {
		serviceRef, ok := serviceRefs[serviceRefDecl.Name]
		if !ok {
			if serviceRefDecl.Optional != nil && *serviceRefDecl.Optional {
				continue
			}
			return fmt.Errorf("service-ref for %s is not defined", serviceRefDecl.Name)
		}

		var (
			namespace = synthesizedComp.Namespace
			sd        *appsv1.ServiceDescriptor
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

type serviceRefReferenceVars struct {
	endpoint *appsv1.CredentialVar
	host     *appsv1.CredentialVar
	port     *appsv1.CredentialVar
	username *appsv1.CredentialVar
	password *appsv1.CredentialVar
	podFQDNs *appsv1.CredentialVar
}

func handleServiceRefFromCluster(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1.ServiceRef, serviceRefDecl appsv1.ServiceRefDeclaration, legacy bool) (*appsv1.ServiceDescriptor, error) {
	resolver := referencedVars
	if legacy {
		resolver = referencedVars4Legacy
	}
	vars := &serviceRefReferenceVars{}
	if err := resolver(ctx, cli, namespace, serviceRef, vars); err != nil {
		return nil, err
	}

	// just in-memory service descriptor object, the namespace and name are trivial
	b := builder.NewServiceDescriptorBuilder(namespace, serviceRefDecl.Name).
		SetServiceVersion("").
		SetServiceKind("")
	setter := func(s func(appsv1.CredentialVar) *builder.ServiceDescriptorBuilder, v *appsv1.CredentialVar) {
		if v != nil {
			s(*v)
		}
	}
	setter(b.SetEndpoint, vars.endpoint)
	setter(b.SetHost, vars.host)
	setter(b.SetPort, vars.port)
	setter(b.SetPodFQDNs, vars.podFQDNs)
	setter(b.SetAuthUsername, vars.username)
	setter(b.SetAuthPassword, vars.password)
	return b.GetObject(), nil
}

func referencedVars(ctx context.Context, cli client.Reader, namespace string, serviceRef appsv1.ServiceRef, vars *serviceRefReferenceVars) error {
	if err := referencedServiceVars(ctx, cli, namespace, serviceRef, vars); err != nil {
		return err
	}
	if err := referencedPodFQDNsVar(ctx, cli, namespace, serviceRef, vars); err != nil {
		return err
	}
	if err := referencedCredentialVars(ctx, cli, namespace, serviceRef, vars); err != nil {
		return err
	}
	return nil
}

func referencedServiceVars(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1.ServiceRef, vars *serviceRefReferenceVars) error {
	var (
		selector = serviceRef.ClusterServiceSelector
		obj      any
		err      error
	)

	if selector.Service == nil {
		return nil
	}

	svcNamespace := namespace
	if serviceRef.Namespace != "" {
		svcNamespace = serviceRef.Namespace
	}
	switch {
	case len(selector.Service.Component) == 0:
		obj, err = clusterServiceGetter(ctx, cli, svcNamespace, selector.Cluster, selector.Service.Service)
	case selector.Service.Service == "headless":
		obj, err = headlessCompServiceGetter(ctx, cli, svcNamespace, selector.Cluster, selector.Service.Component)
	default:
		obj, err = compServiceGetter(ctx, cli, svcNamespace, selector.Cluster, selector.Service.Component, selector.Service.Service)
	}
	if err != nil {
		return err
	}

	// use the service name when the referred service is in the same namespace, to keep it consistent with the vars.
	fqdn := svcNamespace != namespace
	vars.host = &appsv1.CredentialVar{Value: composeHostValueFromServices(obj, fqdn)}
	if p := composePortValueFromServices(obj, selector.Service.Port); p != nil {
		vars.port = &appsv1.CredentialVar{Value: *p}
	}

	vars.endpoint = func() *appsv1.CredentialVar {
		hval := vars.host.Value
		if vars.port == nil {
			return &appsv1.CredentialVar{Value: hval}
		}
		if strings.Contains(hval, ",") {
			// pod-service, the port value has format: host1:port1,host2,port2,...
			return &appsv1.CredentialVar{Value: vars.port.Value}
		}
		return &appsv1.CredentialVar{Value: fmt.Sprintf("%s:%s", hval, vars.port.Value)}
	}()
	return nil
}

func referencedPodFQDNsVar(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1.ServiceRef, vars *serviceRefReferenceVars) error {
	var (
		selector = serviceRef.ClusterServiceSelector
	)
	if selector.PodFQDNs == nil {
		return nil
	}

	if serviceRef.Namespace != "" {
		namespace = serviceRef.Namespace
	}
	var (
		fqdn        string
		err         error
		clusterName = selector.Cluster
		compName    = selector.PodFQDNs.Component
	)
	if selector.PodFQDNs.Role == nil {
		fqdn, err = componentVarPodsGetter(ctx, cli, namespace, clusterName, compName, nil, true)
	} else {
		fqdn, err = componentVarPodsWithRoleGetter(ctx, cli, namespace, clusterName, compName, *selector.PodFQDNs.Role, true)
	}
	if err != nil {
		return err
	}
	vars.podFQDNs = &appsv1.CredentialVar{Value: fqdn}

	return nil
}

func referencedCredentialVars(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1.ServiceRef, vars *serviceRefReferenceVars) error {
	var (
		selector = serviceRef.ClusterServiceSelector
	)

	if selector.Credential == nil {
		return nil
	}

	if len(serviceRef.Namespace) > 0 && serviceRef.Namespace != namespace {
		return fmt.Errorf("prohibits referencing credential variables from different namespaces, service-ref： %s", serviceRef.Name)
	}

	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      constant.GenerateAccountSecretName(selector.Cluster, selector.Credential.Component, selector.Credential.Name),
	}
	secret := &corev1.Secret{}
	if err := cli.Get(ctx, secretKey, secret); err != nil {
		return err
	}

	copySecretDataToCredentialVar(namespace, secret, constant.AccountNameForSecret, &vars.username)
	copySecretDataToCredentialVar(namespace, secret, constant.AccountPasswdForSecret, &vars.password)

	return nil
}

func referencedVars4Legacy(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1.ServiceRef, vars *serviceRefReferenceVars) error {
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
		return err
	}

	copySecretDataToCredentialVar("", secret, constant.ServiceDescriptorEndpointKey, &vars.endpoint)
	copySecretDataToCredentialVar("", secret, constant.ServiceDescriptorPortKey, &vars.port)
	copySecretDataToCredentialVar("", secret, constant.ServiceDescriptorUsernameKey, &vars.username)
	copySecretDataToCredentialVar("", secret, constant.ServiceDescriptorPasswordKey, &vars.password)

	// don't set the host and podFQDNs for legacy clusters
	vars.host = nil
	vars.podFQDNs = nil

	return nil
}

func copySecretDataToCredentialVar(namespace string, secret *corev1.Secret, key string, v **appsv1.CredentialVar) {
	if _, ok := secret.Data[key]; !ok {
		return
	}
	if secret.Namespace == namespace || namespace == "" {
		*v = &appsv1.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
					Key:                  key,
				},
			},
		}
	} else {
		*v = &appsv1.CredentialVar{Value: string(secret.Data[key])}
	}
}

// handleServiceRefFromServiceDescriptor handles the service reference is provided by external ServiceDescriptor object.
func handleServiceRefFromServiceDescriptor(ctx context.Context, cli client.Reader, namespace string,
	serviceRef appsv1.ServiceRef, serviceRefDecl appsv1.ServiceRefDeclaration) (*appsv1.ServiceDescriptor, error) {
	// verify service kind and version
	verifyServiceKindAndVersion := func(serviceDescriptor appsv1.ServiceDescriptor, _ ...appsv1.ServiceRefDeclarationSpec) bool {
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
	serviceDescriptor := &appsv1.ServiceDescriptor{}
	if err := cli.Get(ctx, serviceDescriptorKey, serviceDescriptor); err != nil {
		return nil, err
	}
	if serviceDescriptor.Status.Phase != appsv1.AvailablePhase {
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

func resolveServiceReferences(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent) error {
	for _, serviceDescriptor := range synthesizedComp.ServiceReferences {
		if serviceDescriptor == nil {
			// ingore nil serviceDescriptor
			continue
		}
		// Only support referencing non-credential variables in configuration
		credentialVars := []*appsv1.CredentialVar{
			serviceDescriptor.Spec.Endpoint,
			serviceDescriptor.Spec.Host,
			serviceDescriptor.Spec.Port,
			serviceDescriptor.Spec.PodFQDNs,
		}
		if err := resolveServiceRefCredentialVars(ctx, cli, serviceDescriptor.Namespace, credentialVars...); err != nil {
			return err
		}
	}
	return nil
}

// resolveServiceRefCredentialVars resolves the credentialVar.ValueFrom to the real value
// TODO: currently, we set the valueFrom to the value, which need to be refactored
func resolveServiceRefCredentialVars(ctx context.Context, cli client.Reader,
	namespace string, credentialVars ...*appsv1.CredentialVar) error {
	for _, credentialVar := range credentialVars {
		// TODO: replace the build-in placeholder with the real value
		if credentialVar == nil || credentialVar.Value != "" {
			continue
		}
		// TODO: currently, we set the valueFrom to the value, which need to be refactored
		if credentialVar.ValueFrom != nil {
			if err := resolveSecretRefCredentialVar(ctx, cli, namespace, credentialVar); err != nil {
				return err
			}
			if err := resolveConfigMapRefCredentialVar(ctx, cli, namespace, credentialVar); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSecretRefCredentialVar(ctx context.Context, cli client.Reader,
	namespace string, credentialVar *appsv1.CredentialVar) error {
	if credentialVar.ValueFrom == nil || credentialVar.ValueFrom.SecretKeyRef == nil {
		return nil
	}
	secretName := credentialVar.ValueFrom.SecretKeyRef.Name
	secretKey := credentialVar.ValueFrom.SecretKeyRef.Key
	secretRef := &corev1.Secret{}
	if err := cli.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secretRef); err != nil {
		return err
	}
	runtimeValBytes, ok := secretRef.Data[secretKey]
	if !ok {
		// return fmt.Errorf("couldn't find key %v in Secret %v/%v", secretKey, namespace, secretName)
		return nil
	}
	// Set the valueFrom to the value and clear the valueFrom
	credentialVar.ValueFrom = nil
	credentialVar.Value = string(runtimeValBytes)
	return nil
}

func resolveConfigMapRefCredentialVar(ctx context.Context, cli client.Reader,
	namespace string, credentialVar *appsv1.CredentialVar) error {
	if credentialVar.ValueFrom == nil || credentialVar.ValueFrom.ConfigMapKeyRef == nil {
		return nil
	}
	configMapName := credentialVar.ValueFrom.ConfigMapKeyRef.Name
	configMapKey := credentialVar.ValueFrom.ConfigMapKeyRef.Key
	configMapRef := &corev1.ConfigMap{}
	if err := cli.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: namespace}, configMapRef); err != nil {
		return err
	}
	runtimeValBytes, ok := configMapRef.Data[configMapKey]
	if !ok {
		// return fmt.Errorf("couldn't find key %v in ConfigMap %v/%v", configMapKey, namespace, configMapName)
		return nil
	}
	// Set the valueFrom to the value and clear the valueFrom
	credentialVar.ValueFrom = nil
	credentialVar.Value = runtimeValBytes
	return nil
}
