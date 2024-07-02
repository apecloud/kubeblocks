/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package multiversion

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

// covert appsv1alpha1.component resources to
// - appsv1.component
// - update referenced cmpd & cmpv to appsv1

var (
	compResource = "components"
	compGVR      = appsv1.GroupVersion.WithResource(compResource)
)

func init() {
	hook.RegisterCRDConversion(compGVR, hook.NewNoVersion(1, 0), compHandler(),
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func compHandler() hook.ConversionHandler {
	return &convertor{
		kind:       "Component",
		source:     &compConvertor{},
		target:     &compConvertor{},
		namespaces: []string{"default"}, // TODO: namespaces
	}
}

type compConvertor struct{}

func (c *compConvertor) list(ctx context.Context, cli *versioned.Clientset, namespace string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().Components(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *compConvertor) used(context.Context, *versioned.Clientset, string, string) (bool, error) {
	return true, nil
}

func (c *compConvertor) get(ctx context.Context, cli *versioned.Clientset, namespace, name string) (client.Object, error) {
	return cli.AppsV1().Components(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *compConvertor) convert(source client.Object) []client.Object {
	spec := source.(*appsv1alpha1.Component).Spec
	return []client.Object{
		&appsv1.Component{
			Spec: appsv1.ComponentSpec{
				CompDef:              spec.CompDef,
				ServiceVersion:       spec.ServiceVersion,
				ServiceRefs:          serviceRefs(spec.ServiceRefs),
				Labels:               spec.Labels,
				Annotations:          spec.Annotations,
				Env:                  spec.Env,
				RuntimeClassName:     spec.RuntimeClassName,
				SchedulingPolicy:     schedulingPolicy(spec.SchedulingPolicy),
				Resources:            spec.Resources,
				VolumeClaimTemplates: volumeClaimTemplates(spec.VolumeClaimTemplates),
				Volumes:              spec.Volumes,
				Services:             componentServices(spec.Services),
				SystemAccounts:       componentSystemAccounts(spec.SystemAccounts),
				Replicas:             spec.Replicas,
				Configs:              clusterComponentConfig(spec.Configs),
				ServiceAccountName:   spec.ServiceAccountName,
				TLSConfig:            tlsConfig(spec.TLSConfig),
				Instances:            instanceTemplate(spec.Instances),
				OfflineInstances:     spec.OfflineInstances,
				DisableExporter:      spec.DisableExporter,
			},
		},
	}
}

func serviceRefs(serviceRefs []appsv1alpha1.ServiceRef) []appsv1.ServiceRef {
	if len(serviceRefs) == 0 {
		return nil
	}
	newServiceRefs := make([]appsv1.ServiceRef, 0)
	for i := range serviceRefs {
		newServiceRefs = append(newServiceRefs, appsv1.ServiceRef{
			Name:                   serviceRefs[i].Name,
			Namespace:              serviceRefs[i].Namespace,
			ClusterServiceSelector: serviceRefClusterSelector(serviceRefs[i].ClusterServiceSelector),
			ServiceDescriptor:      serviceRefs[i].ServiceDescriptor,
		})
	}
	return newServiceRefs
}

func serviceRefClusterSelector(selector *appsv1alpha1.ServiceRefClusterSelector) *appsv1.ServiceRefClusterSelector {
	if selector == nil {
		return nil
	}
	newSelector := &appsv1.ServiceRefClusterSelector{
		Cluster: selector.Cluster,
	}
	if selector.Service != nil {
		newSelector.Service = &appsv1.ServiceRefServiceSelector{
			Component: selector.Service.Component,
			Service:   selector.Service.Service,
			Port:      selector.Service.Port,
		}
	}
	if selector.Credential != nil {
		newSelector.Credential = &appsv1.ServiceRefCredentialSelector{
			Component: selector.Credential.Component,
			Name:      selector.Credential.Name,
		}
	}
	return newSelector
}

func schedulingPolicy(policy *appsv1alpha1.SchedulingPolicy) *appsv1.SchedulingPolicy {
	if policy == nil {
		return nil
	}
	return &appsv1.SchedulingPolicy{
		SchedulerName:             policy.SchedulerName,
		NodeSelector:              policy.NodeSelector,
		NodeName:                  policy.NodeName,
		Affinity:                  policy.Affinity,
		Tolerations:               policy.Tolerations,
		TopologySpreadConstraints: policy.TopologySpreadConstraints,
	}
}

func volumeClaimTemplates(vcts []appsv1alpha1.ClusterComponentVolumeClaimTemplate) []appsv1.ClusterComponentVolumeClaimTemplate {
	if len(vcts) == 0 {
		return nil
	}
	newVCTs := make([]appsv1.ClusterComponentVolumeClaimTemplate, 0)
	for i := range vcts {
		newVCTs = append(newVCTs, appsv1.ClusterComponentVolumeClaimTemplate{
			Name: vcts[i].Name,
			Spec: appsv1.PersistentVolumeClaimSpec{
				AccessModes:      vcts[i].Spec.AccessModes,
				Resources:        vcts[i].Spec.Resources,
				StorageClassName: vcts[i].Spec.StorageClassName,
				VolumeMode:       vcts[i].Spec.VolumeMode,
			},
		})
	}
	return newVCTs
}

func componentSystemAccounts(accounts []appsv1alpha1.ComponentSystemAccount) []appsv1.ComponentSystemAccount {
	if len(accounts) == 0 {
		return nil
	}
	newAccounts := make([]appsv1.ComponentSystemAccount, 0)
	for i := range accounts {
		account := appsv1.ComponentSystemAccount{
			Name:     accounts[i].Name,
			Password: appsv1.SystemAccountPassword{},
		}
		if accounts[i].PasswordConfig != nil {
			account.Password.GenerationPolicy = &appsv1.PasswordGenerationPolicy{
				Length:     accounts[i].PasswordConfig.Length,
				NumDigits:  accounts[i].PasswordConfig.NumDigits,
				NumSymbols: accounts[i].PasswordConfig.NumSymbols,
				LetterCase: appsv1.LetterCase(accounts[i].PasswordConfig.LetterCase),
			}
			if len(accounts[i].PasswordConfig.Seed) > 0 {
				account.Password.GenerationPolicy.Seed = &accounts[i].PasswordConfig.Seed
			}
		}
		if accounts[i].SecretRef != nil {
			account.Password.SecretRef = &corev1.SecretReference{
				Name:      accounts[i].SecretRef.Name,
				Namespace: accounts[i].SecretRef.Namespace,
			}
		}
		newAccounts = append(newAccounts, account)
	}
	return newAccounts
}

func clusterComponentConfig(configs []appsv1alpha1.ClusterComponentConfig) []appsv1.ClusterComponentConfig {
	if len(configs) == 0 {
		return nil
	}
	newConfigs := make([]appsv1.ClusterComponentConfig, 0)
	for i := range configs {
		newConfigs = append(newConfigs, appsv1.ClusterComponentConfig{
			Name: configs[i].Name,
			ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
				ConfigMap: configs[i].ConfigMap,
			},
		})
	}
	return newConfigs
}

func tlsConfig(tls *appsv1alpha1.TLSConfig) *appsv1.TLSConfig {
	if tls == nil {
		return nil
	}
	newTLS := &appsv1.TLSConfig{
		Enable: tls.Enable,
	}
	if tls.Issuer != nil {
		newTLS.Issuer = &appsv1.Issuer{
			Name: appsv1.IssuerName(tls.Issuer.Name),
		}
		if tls.Issuer.SecretRef != nil {
			newTLS.Issuer.SecretRef = &appsv1.TLSSecretRef{
				Name: tls.Issuer.SecretRef.Name,
				CA:   tls.Issuer.SecretRef.CA,
				Cert: tls.Issuer.SecretRef.Cert,
				Key:  tls.Issuer.SecretRef.Key,
			}
		}
	}
	return newTLS
}

func instanceTemplate(templates []appsv1alpha1.InstanceTemplate) []appsv1.InstanceTemplate {
	if len(templates) == 0 {
		return nil
	}
	newTemplates := make([]appsv1.InstanceTemplate, 0)
	for i := range templates {
		newTemplates = append(newTemplates, appsv1.InstanceTemplate{
			Name:                 templates[i].Name,
			Replicas:             templates[i].Replicas,
			Annotations:          templates[i].Annotations,
			Labels:               templates[i].Labels,
			Image:                templates[i].Image,
			SchedulingPolicy:     schedulingPolicy(templates[i].SchedulingPolicy),
			Resources:            templates[i].Resources,
			Env:                  templates[i].Env,
			Volumes:              templates[i].Volumes,
			VolumeMounts:         templates[i].VolumeMounts,
			VolumeClaimTemplates: volumeClaimTemplates(templates[i].VolumeClaimTemplates),
		})
	}
	return newTemplates
}
