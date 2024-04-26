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

package operations

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ExposeOpsHandler struct {
}

var _ OpsHandler = ExposeOpsHandler{}

func init() {
	// ToClusterPhase is not defined, because 'expose' does not affect the cluster status.
	exposeBehavior := OpsBehaviour{
		OpsHandler:  ExposeOpsHandler{},
		QueueBySelf: true,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.ExposeType, exposeBehavior)
}

func (e ExposeOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		exposeMap = opsRes.OpsRequest.Spec.ToExposeListToMap()
	)
	reqCtx.Log.Info("cluster service before action", "clusterService", opsRes.Cluster.Spec.Services)
	compMap := make(map[string]appsv1alpha1.ClusterComponentSpec)
	for _, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		compMap[comp.Name] = comp
	}

	for _, expose := range exposeMap {
		clusterCompSpecName := ""
		clusterCompDef := ""
		clusterCompDefRefName := ""
		if len(expose.ComponentName) > 0 {
			clusterCompSpec, ok := compMap[expose.ComponentName]
			if !ok {
				return fmt.Errorf("component spec not found: %s", expose.ComponentName)
			}
			clusterCompSpecName = clusterCompSpec.Name
			clusterCompDef = clusterCompSpec.ComponentDef
			clusterCompDefRefName = clusterCompSpec.ComponentDefRef
		}

		switch expose.Switch {
		case appsv1alpha1.EnableExposeSwitch:
			if err := e.buildClusterServices(reqCtx, cli, opsRes.Cluster, clusterCompSpecName, clusterCompDef, clusterCompDefRefName, expose.Services); err != nil {
				return err
			}
		case appsv1alpha1.DisableExposeSwitch:
			if err := e.removeClusterServices(opsRes.Cluster, clusterCompSpecName, expose.Services); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid expose switch: %s", expose.Switch)
		}
	}
	reqCtx.Log.Info("cluster service to be updated", "clusterService", opsRes.Cluster.Spec.Services)
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

func (e ExposeOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequest          = opsResource.OpsRequest
		oldOpsRequestStatus = opsRequest.Status.DeepCopy()
		opsRequestPhase     = appsv1alpha1.OpsRunningPhase
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())

	// update component status
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = make(map[string]appsv1alpha1.OpsRequestComponentStatus)
		for _, v := range opsRequest.Spec.ExposeList {
			opsRequest.Status.Components[v.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{
				Phase: appsv1alpha1.UpdatingClusterCompPhase, // appsv1alpha1.ExposingPhase,
			}
		}
	}

	var (
		actualProgressCount int
		expectProgressCount int
	)
	for _, v := range opsRequest.Spec.ExposeList {
		actualCount, expectCount, err := e.handleComponentServices(reqCtx, cli, opsResource, v)
		if err != nil {
			return "", 0, err
		}
		actualProgressCount += actualCount
		expectProgressCount += expectCount

		// update component status if completed
		if actualCount == expectCount {
			p := opsRequest.Status.Components[v.ComponentName]
			p.Phase = appsv1alpha1.RunningClusterCompPhase
		}
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", actualProgressCount, expectProgressCount)

	// patch OpsRequest.status.components
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}

	if actualProgressCount == expectProgressCount {
		opsRequestPhase = appsv1alpha1.OpsSucceedPhase
	}

	return opsRequestPhase, 5 * time.Second, nil
}

func (e ExposeOpsHandler) handleComponentServices(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, expose appsv1alpha1.Expose) (int, int, error) {
	svcList := &corev1.ServiceList{}
	if err := cli.List(reqCtx.Ctx, svcList, client.MatchingLabels{
		constant.AppInstanceLabelKey: opsRes.Cluster.Name,
	}, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return 0, 0, err
	}

	getSvcName := func(clusterName string, componentName string, name string) string {
		parts := []string{clusterName}
		if componentName != "" {
			parts = append(parts, componentName)
		}
		if name != "" {
			parts = append(parts, name)
		}
		return strings.Join(parts, "-")
	}

	var (
		svcMap         = make(map[string]corev1.Service)
		defaultSvcName = getSvcName(opsRes.Cluster.Name, expose.ComponentName, "")
	)
	for _, svc := range svcList.Items {
		if svc.Name == defaultSvcName {
			continue
		}
		svcMap[svc.Name] = svc
	}

	var (
		expectCount = len(expose.Services)
		actualCount int
	)

	checkEnableExposeService := func() {
		for _, item := range expose.Services {
			service, ok := svcMap[getSvcName(opsRes.Cluster.Name, expose.ComponentName, item.Name)]
			if !ok {
				continue
			}

			if item.ServiceType == corev1.ServiceTypeLoadBalancer {
				for _, ingress := range service.Status.LoadBalancer.Ingress {
					if ingress.Hostname == "" && ingress.IP == "" {
						continue
					}
					actualCount += 1
					break
				}
			} else {
				actualCount += 1
			}
		}
	}

	checkDisableExposeService := func() {
		for _, item := range expose.Services {
			_, ok := svcMap[getSvcName(opsRes.Cluster.Name, expose.ComponentName, item.Name)]
			// if service is not found, it means that the service has been removed
			if !ok {
				actualCount += 1
			}
		}
	}

	switch expose.Switch {
	case appsv1alpha1.EnableExposeSwitch:
		checkEnableExposeService()
	case appsv1alpha1.DisableExposeSwitch:
		checkDisableExposeService()
	}

	return actualCount, expectCount, nil
}

func (e ExposeOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewExposingCondition(opsRes.OpsRequest), nil
}

func (e ExposeOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	compOpsSet := map[string]sets.Empty{}
	for _, v := range opsResource.OpsRequest.Spec.ExposeList {
		if v.ComponentName != "" {
			compOpsSet[v.ComponentName] = sets.Empty{}
		}
	}
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsResource.Cluster.Spec.ComponentSpecs {
		if _, ok := compOpsSet[v.Name]; !ok {
			continue
		}
		lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
			Services: v.Services,
		}
	}
	opsResource.OpsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

func (e ExposeOpsHandler) removeClusterServices(cluster *appsv1alpha1.Cluster,
	clusterCompSpecName string,
	exposeServices []appsv1alpha1.OpsService) error {
	if cluster == nil || len(exposeServices) == 0 {
		return nil
	}
	for _, exposeService := range exposeServices {
		genServiceName := generateServiceName(clusterCompSpecName, exposeService.Name)
		for i, clusterService := range cluster.Spec.Services {
			// remove service from cluster
			if clusterService.Name == genServiceName && clusterService.ComponentSelector == clusterCompSpecName {
				cluster.Spec.Services = append(cluster.Spec.Services[:i], cluster.Spec.Services[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (e ExposeOpsHandler) buildClusterServices(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpecName string,
	clusterCompDefName string,
	clusterCompDefRefName string,
	exposeServices []appsv1alpha1.OpsService) error {
	if cluster == nil || len(exposeServices) == 0 {
		return nil
	}

	checkServiceExist := func(exposeService appsv1alpha1.OpsService) bool {
		if len(cluster.Spec.Services) == 0 {
			return false
		}

		genServiceName := generateServiceName(clusterCompSpecName, exposeService.Name)

		for _, clusterService := range cluster.Spec.Services {
			if clusterService.ComponentSelector != clusterCompSpecName {
				continue
			}
			if clusterService.Name == genServiceName {
				return true
			}
		}
		return false
	}

	convertDefaultCompDefServicePorts := func(compServices []appsv1alpha1.ComponentService) ([]corev1.ServicePort, error) {
		if len(compServices) == 0 {
			return nil, fmt.Errorf("component service is not defined, expose operation is not supported, cluster: %s, component: %s", cluster.Name, clusterCompSpecName)
		}
		defaultServicePorts := make([]corev1.ServicePort, 0, len(compServices))
		portsSet := make(map[string]bool) // to avoid duplicate ports
		for _, compService := range compServices {
			if compService.Spec.Type == corev1.ServiceTypeLoadBalancer || compService.Spec.Type == corev1.ServiceTypeNodePort {
				continue
			}
			for _, p := range compService.Spec.Ports {
				// port key is in the format of `protocol-port`, eg: TCP-80
				portKey := fmt.Sprintf("%s-%d", p.Protocol, p.Port)
				if _, ok := portsSet[portKey]; ok {
					continue
				}
				portsSet[portKey] = true
				genServicePort := corev1.ServicePort{
					Name:        p.Name,
					Protocol:    p.Protocol,
					AppProtocol: p.AppProtocol,
					Port:        p.Port,
					TargetPort:  p.TargetPort,
				}
				defaultServicePorts = append(defaultServicePorts, genServicePort)
			}
		}
		if len(defaultServicePorts) == 0 {
			return nil, fmt.Errorf("component does not define an available service, expose operation is not supported, cluster: %s, component: %s", cluster.Name, clusterCompSpecName)
		}
		return defaultServicePorts, nil
	}

	defaultServicePortsFunc := func() ([]corev1.ServicePort, error) {
		if clusterCompDefName != "" {
			compDef, err := component.GetCompDefinition(reqCtx, cli, cluster, clusterCompSpecName)
			if err != nil {
				return nil, err
			}
			return convertDefaultCompDefServicePorts(compDef.Spec.Services)
		}
		if cluster.Spec.ClusterDefRef != "" && clusterCompDefRefName != "" {
			clusterDef, err := getClusterDefByName(reqCtx.Ctx, cli, cluster.Spec.ClusterDefRef)
			if err != nil {
				return nil, err
			}
			clusterCompDef := clusterDef.GetComponentDefByName(clusterCompDefRefName)
			if clusterCompDef == nil || clusterCompDef.Service == nil {
				return nil, fmt.Errorf("referenced cluster component definition or services is not defined: %s", clusterCompDefRefName)
			}
			return clusterCompDef.Service.ToSVCPorts(), nil
		}
		return nil, fmt.Errorf("component definition is not defined, cluster: %s, component: %s", cluster.Name, clusterCompSpecName)
	}

	defaultRoleSelectorFunc := func() (string, error) {
		if clusterCompDefName != "" {
			compDef, err := component.GetCompDefinition(reqCtx, cli, cluster, clusterCompSpecName)
			if err != nil {
				return "", err
			}
			if len(compDef.Spec.Roles) == 0 {
				return "", nil
			}
			for _, role := range compDef.Spec.Roles {
				if role.Writable && role.Serviceable {
					return role.Name, nil
				}
			}
			return "", nil
		}
		if cluster.Spec.ClusterDefRef != "" && clusterCompDefRefName != "" {
			clusterDef, err := getClusterDefByName(reqCtx.Ctx, cli, cluster.Spec.ClusterDefRef)
			if err != nil {
				return "", err
			}
			clusterCompDef := clusterDef.GetComponentDefByName(clusterCompDefRefName)
			if clusterCompDef == nil {
				return "", fmt.Errorf("referenced cluster component definition is not defined: %s", clusterCompDefRefName)
			}
			switch clusterCompDef.WorkloadType {
			case appsv1alpha1.Replication:
				return constant.Primary, nil
			case appsv1alpha1.Consensus:
				if clusterCompDef.ConsensusSpec != nil {
					return clusterCompDef.ConsensusSpec.Leader.Name, nil
				}
				return constant.Leader, nil
			case appsv1alpha1.Stateful:
				if clusterCompDef.RSMSpec != nil {
					for _, role := range clusterCompDef.RSMSpec.Roles {
						if role.IsLeader {
							return role.Name, nil
						}
					}
				}
			}
		}
		return "", nil
	}

	for _, exposeService := range exposeServices {
		if checkServiceExist(exposeService) {
			reqCtx.Log.Info("cluster service already exists, skip", "service", exposeService.Name)
			continue
		}

		genServiceName := generateServiceName(clusterCompSpecName, exposeService.Name)

		clusterService := appsv1alpha1.ClusterService{
			Service: appsv1alpha1.Service{
				Name:        genServiceName,
				ServiceName: genServiceName,
				Annotations: exposeService.Annotations,
				Spec: corev1.ServiceSpec{
					Type: exposeService.ServiceType,
				},
			},
			ComponentSelector: clusterCompSpecName,
		}

		// set service selector
		if exposeService.PodSelector != nil {
			clusterService.Spec.Selector = exposeService.PodSelector
		}

		// set service ports
		if len(exposeService.Ports) != 0 {
			clusterService.Spec.Ports = exposeService.Ports
		} else {
			defaultServicePorts, err := defaultServicePortsFunc()
			if err != nil {
				return err
			}
			clusterService.Spec.Ports = defaultServicePorts
		}

		// set ip family policy
		if exposeService.IPFamilyPolicy != nil {
			clusterService.Spec.IPFamilyPolicy = exposeService.IPFamilyPolicy
		}

		// set ip families
		if len(exposeService.IPFamilies) > 0 {
			clusterService.Spec.IPFamilies = exposeService.IPFamilies
		}

		// set role selector
		if len(exposeService.RoleSelector) != 0 {
			clusterService.RoleSelector = exposeService.RoleSelector
		} else if len(clusterCompSpecName) > 0 {
			defaultRoleSelector, err := defaultRoleSelectorFunc()
			if err != nil {
				return err
			}
			if defaultRoleSelector != "" {
				clusterService.RoleSelector = defaultRoleSelector
			}
		}
		cluster.Spec.Services = append(cluster.Spec.Services, clusterService)
	}
	return nil
}

func generateServiceName(clusterCompSpecName, exposeServiceName string) string {
	if len(clusterCompSpecName) > 0 {
		return fmt.Sprintf("%s-%s", clusterCompSpecName, exposeServiceName)
	}
	return exposeServiceName
}
