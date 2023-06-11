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

package operations

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ExposeOpsHandler struct {
}

var _ OpsHandler = ExposeOpsHandler{}

func init() {
	// ToClusterPhase is not defined, because 'expose' does not affect the cluster status.
	exposeBehavior := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		OpsHandler:        ExposeOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.ExposeType, exposeBehavior)
}

func (e ExposeOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		exposeMap = opsRes.OpsRequest.Spec.ToExposeListToMap()
	)

	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		expose, ok := exposeMap[component.Name]
		if !ok {
			continue
		}
		opsRes.Cluster.Spec.ComponentSpecs[index].Services = expose.Services
	}
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
				Phase: appsv1alpha1.SpecReconcilingClusterCompPhase, // appsv1alpha1.ExposingPhase,
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
		constant.AppInstanceLabelKey:    opsRes.Cluster.Name,
		constant.KBAppComponentLabelKey: expose.ComponentName,
	}, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return 0, 0, err
	}

	getSvcName := func(clusterName string, componentName string, name string) string {
		parts := []string{clusterName, componentName}
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

	for _, item := range expose.Services {
		service, ok := svcMap[getSvcName(opsRes.Cluster.Name, expose.ComponentName, item.Name)]
		if !ok {
			continue
		}

		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.Hostname == "" && ingress.IP == "" {
				continue
			}
			actualCount += 1
			break
		}
	}

	return actualCount, expectCount, nil
}

func (e ExposeOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewExposingCondition(opsRequest)
}

func (e ExposeOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	componentNameSet := opsResource.OpsRequest.GetComponentNameSet()
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsResource.Cluster.Spec.ComponentSpecs {
		if _, ok := componentNameSet[v.Name]; !ok {
			continue
		}
		lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
			Services: v.Services,
		}
	}
	opsResource.OpsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

func (e ExposeOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return realAffectedComponentMap(opsRequest.Spec.GetExposeComponentNameSet())
}
