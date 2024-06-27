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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

// covert appsv1alpha1.cluster resources to
// - appsv1.cluster
// - update referenced cd & cmpd to appsv1

var (
	clusterResource = "clusters"
	clusterGVR      = appsv1.GroupVersion.WithResource(clusterResource)
)

func init() {
	hook.RegisterCRDConversion(clusterGVR, hook.NewNoVersion(1, 0), clusterHandler(),
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func clusterHandler() hook.ConversionHandler {
	return &convertor{
		namespaces: []string{"default"}, // TODO: namespaces
		sourceKind: &clusterConvertor{},
		targetKind: &clusterConvertor{},
	}
}

type clusterConvertor struct{}

func (c *clusterConvertor) kind() string {
	return "Cluster"
}

func (c *clusterConvertor) list(ctx context.Context, cli *versioned.Clientset, namespace string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().Clusters(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *clusterConvertor) get(ctx context.Context, cli *versioned.Clientset, namespace, name string) (client.Object, error) {
	return cli.AppsV1().Clusters(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *clusterConvertor) convert(source client.Object) []client.Object {
	spec := source.(*appsv1alpha1.Cluster).Spec
	return []client.Object{
		&appsv1.Cluster{
			Spec: appsv1.ClusterSpec{
				ClusterDefRef:     spec.ClusterDefRef,
				Topology:          spec.Topology,
				TerminationPolicy: appsv1.TerminationPolicyType(spec.TerminationPolicy),
				ShardingSpecs:     c.shardings(spec.ShardingSpecs),
				ComponentSpecs:    c.components(spec.ComponentSpecs),
				RuntimeClassName:  spec.RuntimeClassName,
				SchedulingPolicy:  schedulingPolicy(spec.SchedulingPolicy),
				Services:          c.services(spec.Services),
				Backup:            c.backup(spec.Backup),
			},
		},
	}
}

func (c *clusterConvertor) shardings(shardings []appsv1alpha1.ShardingSpec) []appsv1.ShardingSpec {
	if len(shardings) == 0 {
		return nil
	}
	newShardings := make([]appsv1.ShardingSpec, 0)
	for i := range shardings {
		newShardings = append(newShardings, appsv1.ShardingSpec{
			Name:     shardings[i].Name,
			Template: c.componentTemplate(shardings[i].Template),
			Shards:   shardings[i].Shards,
		})
	}
	return newShardings
}

func (c *clusterConvertor) components(comps []appsv1alpha1.ClusterComponentSpec) []appsv1.ClusterComponentSpec {
	if len(comps) == 0 {
		return nil
	}
	newComps := make([]appsv1.ClusterComponentSpec, 0)
	for i := range comps {
		newComp := c.componentTemplate(comps[i])
		newComp.Name = comps[i].Name
		newComps = append(newComps, newComp)
	}
	return newComps
}

func (c *clusterConvertor) componentTemplate(template appsv1alpha1.ClusterComponentSpec) appsv1.ClusterComponentSpec {
	newTemplate := appsv1.ClusterComponentSpec{
		ComponentDef:         template.ComponentDef,
		ServiceVersion:       template.ServiceVersion,
		ServiceRefs:          serviceRefs(template.ServiceRefs),
		Labels:               template.Labels,
		Annotations:          template.Annotations,
		Env:                  template.Env,
		Replicas:             template.Replicas,
		SchedulingPolicy:     schedulingPolicy(template.SchedulingPolicy),
		Resources:            template.Resources,
		VolumeClaimTemplates: volumeClaimTemplates(template.VolumeClaimTemplates),
		Volumes:              template.Volumes,
		Services:             c.clusterComponentServices(template.Services),
		SystemAccounts:       componentSystemAccounts(template.SystemAccounts),
		Configs:              clusterComponentConfig(template.Configs),
		TLSConfig:            tlsConfig(&appsv1alpha1.TLSConfig{Enable: template.TLS, Issuer: template.Issuer}),
		ServiceAccountName:   template.ServiceAccountName,
		Instances:            instanceTemplate(template.Instances),
		OfflineInstances:     template.OfflineInstances,
		DisableExporter:      template.DisableExporter,
	}
	return newTemplate
}

func (c *clusterConvertor) clusterComponentServices(services []appsv1alpha1.ClusterComponentService) []appsv1.ClusterComponentService {
	if len(services) == 0 {
		return nil
	}
	newServices := make([]appsv1.ClusterComponentService, 0)
	for i := range services {
		newServices = append(newServices, appsv1.ClusterComponentService{
			Name:        services[i].Name,
			ServiceType: services[i].ServiceType,
			Annotations: services[i].Annotations,
			PodService:  services[i].PodService,
		})
	}
	return newServices
}

func (c *clusterConvertor) services(services []appsv1alpha1.ClusterService) []appsv1.ClusterService {
	if len(services) == 0 {
		return nil
	}
	newServices := make([]appsv1.ClusterService, 0)
	for i := range services {
		newServices = append(newServices, appsv1.ClusterService{
			Service: appsv1.Service{
				Name:         services[i].Name,
				ServiceName:  services[i].ServiceName,
				Annotations:  services[i].Annotations,
				Spec:         services[i].Spec,
				RoleSelector: services[i].RoleSelector,
			},
			ShardingSelector:  services[i].ShardingSelector,
			ComponentSelector: services[i].ComponentSelector,
		})
	}
	return newServices
}

func (c *clusterConvertor) backup(backup *appsv1alpha1.ClusterBackup) *appsv1.ClusterBackup {
	if backup != nil {
		return nil
	}
	return &appsv1.ClusterBackup{
		Enabled:                 backup.Enabled,
		RetentionPeriod:         string(backup.RetentionPeriod),
		Method:                  backup.Method,
		CronExpression:          backup.CronExpression,
		StartingDeadlineMinutes: backup.StartingDeadlineMinutes,
		RepoName:                backup.RepoName,
		PITREnabled:             backup.PITREnabled,
	}
}
