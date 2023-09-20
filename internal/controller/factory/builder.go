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

package factory

import (
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/leaanthony/debme"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	VolumeName = "tls"
	CAName     = "ca.crt"
	CertName   = "tls.crt"
	KeyName    = "tls.key"
	MountPath  = "/etc/pki/tls"
)

var (
	//go:embed cue/*
	cueTemplates embed.FS
	cacheCtx     = map[string]interface{}{}
)

func getCacheCUETplValue(key string, valueCreator func() (*intctrlutil.CUETpl, error)) (*intctrlutil.CUETpl, error) {
	vIf, ok := cacheCtx[key]
	if ok {
		return vIf.(*intctrlutil.CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	cacheCtx[key] = v
	return v, err
}

func buildFromCUE(tplName string, fillMap map[string]any, lookupKey string, target any) error {
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := getCacheCUETplValue(tplName, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplName))
	})
	if err != nil {
		return err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	for k, v := range fillMap {
		if err := cueValue.FillObj(k, v); err != nil {
			return err
		}
	}

	b, err := cueValue.Lookup(lookupKey)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(b, target); err != nil {
		return err
	}

	return nil
}

func processContainersInjection(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	envConfigName string,
	podSpec *corev1.PodSpec) error {
	for _, cc := range []*[]corev1.Container{
		&podSpec.Containers,
		&podSpec.InitContainers,
	} {
		for i := range *cc {
			if err := injectEnvs(cluster, component, envConfigName, &(*cc)[i]); err != nil {
				return err
			}
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&(*cc)[i])
		}
	}
	return nil
}

func injectEnvs(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent, envConfigName string, c *corev1.Container) error {
	// can not use map, it is unordered
	envFieldPathSlice := []struct {
		name      string
		fieldPath string
	}{
		{name: constant.KBEnvPodName, fieldPath: "metadata.name"},
		{name: constant.KBEnvPodUID, fieldPath: "metadata.uid"},
		{name: constant.KBEnvNamespace, fieldPath: "metadata.namespace"},
		{name: "KB_SA_NAME", fieldPath: "spec.serviceAccountName"},
		{name: constant.KBEnvNodeName, fieldPath: "spec.nodeName"},
		{name: constant.KBEnvHostIP, fieldPath: "status.hostIP"},
		{name: "KB_POD_IP", fieldPath: "status.podIP"},
		{name: "KB_POD_IPS", fieldPath: "status.podIPs"},
		// TODO: need to deprecate following
		{name: "KB_HOSTIP", fieldPath: "status.hostIP"},
		{name: "KB_PODIP", fieldPath: "status.podIP"},
		{name: "KB_PODIPS", fieldPath: "status.podIPs"},
	}

	toInjectEnvs := make([]corev1.EnvVar, 0, len(envFieldPathSlice)+len(c.Env))
	for _, v := range envFieldPathSlice {
		toInjectEnvs = append(toInjectEnvs, corev1.EnvVar{
			Name: v.name,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  v.fieldPath,
				},
			},
		})
	}

	var kbClusterPostfix8 string
	if len(cluster.UID) > 8 {
		kbClusterPostfix8 = string(cluster.UID)[len(cluster.UID)-8:]
	} else {
		kbClusterPostfix8 = string(cluster.UID)
	}
	toInjectEnvs = append(toInjectEnvs, []corev1.EnvVar{
		{Name: "KB_CLUSTER_NAME", Value: cluster.Name},
		{Name: "KB_COMP_NAME", Value: component.Name},
		{Name: "KB_CLUSTER_COMP_NAME", Value: cluster.Name + "-" + component.Name},
		{Name: "KB_CLUSTER_UID_POSTFIX_8", Value: kbClusterPostfix8},
		{Name: "KB_POD_FQDN", Value: fmt.Sprintf("%s.%s-headless.%s.svc", "$(KB_POD_NAME)",
			"$(KB_CLUSTER_COMP_NAME)", "$(KB_NAMESPACE)")},
	}...)

	if component.TLS {
		toInjectEnvs = append(toInjectEnvs, []corev1.EnvVar{
			{Name: "KB_TLS_CERT_PATH", Value: MountPath},
			{Name: "KB_TLS_CA_FILE", Value: CAName},
			{Name: "KB_TLS_CERT_FILE", Value: CertName},
			{Name: "KB_TLS_KEY_FILE", Value: KeyName},
		}...)
	}

	if udeValue, ok := cluster.Annotations[constant.ExtraEnvAnnotationKey]; ok {
		udeMap := make(map[string]string)
		if err := json.Unmarshal([]byte(udeValue), &udeMap); err != nil {
			return err
		}
		keys := make([]string, 0)
		for k := range udeMap {
			if k == "" || udeMap[k] == "" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			toInjectEnvs = append(toInjectEnvs, corev1.EnvVar{
				Name:  k,
				Value: udeMap[k],
			})
		}
	}

	// have injected variables placed at the front of the slice
	if len(c.Env) == 0 {
		c.Env = toInjectEnvs
	} else {
		c.Env = append(toInjectEnvs, c.Env...)
	}
	if envConfigName == "" {
		return nil
	}
	c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: envConfigName,
			},
		},
	})
	return nil
}

// BuildPersistentVolumeClaimLabels builds a pvc name label, and synchronize the labels from sts to pvc.
func BuildPersistentVolumeClaimLabels(component *component.SynthesizedComponent, pvc *corev1.PersistentVolumeClaim,
	pvcTplName string) {
	// strict args checking.
	if pvc == nil || component == nil {
		return
	}
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = pvcTplName

	if component.VolumeTypes != nil {
		for _, t := range component.VolumeTypes {
			if t.Name == pvcTplName {
				pvc.Labels[constant.VolumeTypeLabelKey] = string(t.Type)
				break
			}
		}
	}
}

func BuildSvcListWithCustomAttributes(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent,
	customAttributeSetter func(*corev1.Service)) ([]*corev1.Service, error) {
	services := BuildSvcList(cluster, component)
	if customAttributeSetter != nil {
		for _, svc := range services {
			customAttributeSetter(svc)
		}
	}
	return services, nil
}

func BuildSvcList(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) []*corev1.Service {
	wellKnownLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	wellKnownLabels[constant.AppComponentLabelKey] = component.CompDefName
	selectors := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	delete(selectors, constant.AppNameLabelKey)
	var result = make([]*corev1.Service, 0)
	for _, item := range component.Services {
		if len(item.Spec.Ports) == 0 {
			continue
		}
		name := fmt.Sprintf("%s-%s", cluster.Name, component.Name)
		if len(item.Name) > 0 {
			name = fmt.Sprintf("%s-%s-%s", cluster.Name, component.Name, item.Name)
		}

		svcBuilder := builder.NewServiceBuilder(cluster.Namespace, name).
			AddLabelsInMap(wellKnownLabels).
			AddAnnotationsInMap(item.Annotations).
			AddSelectorsInMap(selectors).
			AddPorts(item.Spec.Ports...)
		if len(item.Spec.Type) > 0 {
			svcBuilder.SetType(item.Spec.Type)
		}
		svc := svcBuilder.GetObject()
		result = append(result, svc)
	}
	return result
}

func BuildHeadlessSvc(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) *corev1.Service {
	wellKnownLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	wellKnownLabels[constant.AppComponentLabelKey] = component.CompDefName
	monitorAnnotations := func() map[string]string {
		annotations := make(map[string]string, 0)
		falseStr := "false"
		trueStr := "true"
		switch {
		case !component.Monitor.Enable:
			annotations["monitor.kubeblocks.io/scrape"] = falseStr
			annotations["monitor.kubeblocks.io/agamotto"] = falseStr
		case component.Monitor.BuiltIn:
			annotations["monitor.kubeblocks.io/scrape"] = falseStr
			annotations["monitor.kubeblocks.io/agamotto"] = trueStr
		default:
			annotations["monitor.kubeblocks.io/scrape"] = trueStr
			annotations["monitor.kubeblocks.io/path"] = component.Monitor.ScrapePath
			annotations["monitor.kubeblocks.io/port"] = strconv.Itoa(int(component.Monitor.ScrapePort))
			annotations["monitor.kubeblocks.io/scheme"] = "http"
			annotations["monitor.kubeblocks.io/agamotto"] = falseStr
		}
		return annotations
	}()
	servicePorts := func() []corev1.ServicePort {
		var servicePorts []corev1.ServicePort
		for _, container := range component.PodSpec.Containers {
			for _, port := range container.Ports {
				servicePort := corev1.ServicePort{
					Name:       port.Name,
					Protocol:   port.Protocol,
					Port:       port.ContainerPort,
					TargetPort: intstr.FromString(port.Name),
				}
				servicePorts = append(servicePorts, servicePort)
			}
		}
		return servicePorts
	}()
	return builder.NewHeadlessServiceBuilder(cluster.Namespace, fmt.Sprintf("%s-%s-headless", cluster.Name, component.Name)).
		AddLabelsInMap(wellKnownLabels).
		AddAnnotationsInMap(monitorAnnotations).
		AddSelector(constant.AppInstanceLabelKey, cluster.Name).
		AddSelector(constant.AppManagedByLabelKey, constant.AppName).
		AddSelector(constant.KBAppComponentLabelKey, component.Name).
		AddPorts(servicePorts...).
		GetObject()
}

func BuildSts(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent, envConfigName string) (*appsv1.StatefulSet, error) {
	vctToPVC := func(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
		return corev1.PersistentVolumeClaim{
			ObjectMeta: vct.ObjectMeta,
			Spec:       vct.Spec,
		}
	}

	commonLabels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppNameLabelKey:        component.ClusterDefName,
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: component.Name,
	}
	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels).
		AddLabels(constant.AppComponentLabelKey, component.CompDefName).
		AddLabels(constant.WorkloadTypeLabelKey, string(component.WorkloadType))
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		podBuilder.AddLabels(constant.AppVersionLabelKey, cluster.Spec.ClusterVersionRef)
	}
	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *component.PodSpec,
	}
	stsBuilder := builder.NewStatefulSetBuilder(cluster.Namespace, cluster.Name+"-"+component.Name).
		AddLabelsInMap(commonLabels).
		AddLabels(constant.AppComponentLabelKey, component.CompDefName).
		AddMatchLabelsInMap(commonLabels).
		SetServiceName(cluster.Name + "-" + component.Name + "-headless").
		SetReplicas(component.Replicas).
		SetTemplate(template)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range component.VolumeClaimTemplates {
		vcts = append(vcts, vctToPVC(vct))
	}
	stsBuilder.SetVolumeClaimTemplates(vcts...)

	if component.StatefulSetWorkload != nil {
		podManagementPolicy, updateStrategy := component.StatefulSetWorkload.FinalStsUpdateStrategy()
		stsBuilder.SetPodManagementPolicy(podManagementPolicy).SetUpdateStrategy(updateStrategy)
	}

	sts := stsBuilder.GetObject()

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(sts.Spec.VolumeClaimTemplates) > 0 && len(sts.GetLabels()) > 0 {
		for index, vct := range sts.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(component, &vct, vct.Name)
			sts.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(reqCtx, cluster, component, envConfigName, &sts.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return sts, nil
}

func buildWellKnownLabels(clusterDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppNameLabelKey:        clusterDefName,
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
	}
}

func BuildRSM(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent, envConfigName string) (*workloads.ReplicatedStateMachine, error) {
	vctToPVC := func(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
		return corev1.PersistentVolumeClaim{
			ObjectMeta: vct.ObjectMeta,
			Spec:       vct.Spec,
		}
	}

	commonLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	addCommonLabels := func(service *corev1.Service) {
		if service == nil {
			return
		}
		labels := service.Labels
		if labels == nil {
			labels = make(map[string]string, 0)
		}
		for k, v := range commonLabels {
			labels[k] = v
		}
		labels[constant.AppComponentLabelKey] = component.CompDefName
		service.Labels = labels
	}

	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels).
		AddLabels(constant.AppComponentLabelKey, component.CompDefName).
		AddLabels(constant.WorkloadTypeLabelKey, string(component.WorkloadType))
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		podBuilder.AddLabels(constant.AppVersionLabelKey, cluster.Spec.ClusterVersionRef)
	}
	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *component.PodSpec,
	}

	monitorAnnotations := func() map[string]string {
		annotations := make(map[string]string, 0)
		falseStr := "false"
		trueStr := "true"
		switch {
		case !component.Monitor.Enable:
			annotations["monitor.kubeblocks.io/scrape"] = falseStr
			annotations["monitor.kubeblocks.io/agamotto"] = falseStr
		case component.Monitor.BuiltIn:
			annotations["monitor.kubeblocks.io/scrape"] = falseStr
			annotations["monitor.kubeblocks.io/agamotto"] = trueStr
		default:
			annotations["monitor.kubeblocks.io/scrape"] = trueStr
			annotations["monitor.kubeblocks.io/path"] = component.Monitor.ScrapePath
			annotations["monitor.kubeblocks.io/port"] = strconv.Itoa(int(component.Monitor.ScrapePort))
			annotations["monitor.kubeblocks.io/scheme"] = "http"
			annotations["monitor.kubeblocks.io/agamotto"] = falseStr
		}
		return rsm.AddAnnotationScope(rsm.HeadlessServiceScope, annotations)
	}()
	rsmName := fmt.Sprintf("%s-%s", cluster.Name, component.Name)
	rsmBuilder := builder.NewReplicatedStateMachineBuilder(cluster.Namespace, rsmName).
		AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
		AddAnnotationsInMap(monitorAnnotations).
		AddLabelsInMap(commonLabels).
		AddLabels(constant.AppComponentLabelKey, component.CompDefName).
		AddMatchLabelsInMap(commonLabels).
		SetServiceName(rsmName + "-headless").
		SetReplicas(component.Replicas).
		SetTemplate(template)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range component.VolumeClaimTemplates {
		vcts = append(vcts, vctToPVC(vct))
	}
	rsmBuilder.SetVolumeClaimTemplates(vcts...)

	if component.StatefulSetWorkload != nil {
		podManagementPolicy, updateStrategy := component.StatefulSetWorkload.FinalStsUpdateStrategy()
		rsmBuilder.SetPodManagementPolicy(podManagementPolicy).SetUpdateStrategy(updateStrategy)
	}

	service, alternativeServices := separateServices(component.Services)
	addCommonLabels(service)
	for i := range alternativeServices {
		addCommonLabels(&alternativeServices[i])
	}
	if service != nil {
		rsmBuilder.SetService(service)
	}
	if len(alternativeServices) == 0 {
		alternativeServices = nil
	}
	alternativeServices = fixService(cluster.Namespace, rsmName, component, alternativeServices...)
	rsmBuilder.SetAlternativeServices(alternativeServices)

	secretName := fmt.Sprintf("%s-conn-credential", cluster.Name)
	credential := workloads.Credential{
		Username: workloads.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountNameForSecret,
				},
			},
		},
		Password: workloads.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountPasswdForSecret,
				},
			},
		},
	}
	rsmBuilder.SetCredential(credential)

	roles, roleProbe, membershipReconfiguration, memberUpdateStrategy := buildRoleInfo(component)
	rsm := rsmBuilder.SetRoles(roles).
		SetRoleProbe(roleProbe).
		SetMembershipReconfiguration(membershipReconfiguration).
		SetMemberUpdateStrategy(memberUpdateStrategy).
		GetObject()

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(rsm.Spec.VolumeClaimTemplates) > 0 && len(rsm.GetLabels()) > 0 {
		for index, vct := range rsm.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(component, &vct, vct.Name)
			rsm.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(reqCtx, cluster, component, envConfigName, &rsm.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return rsm, nil
}

func fixService(namespace, prefix string, component *component.SynthesizedComponent, alternativeServices ...corev1.Service) []corev1.Service {
	leaderName := getLeaderName(component)
	for i := range alternativeServices {
		if len(alternativeServices[i].Name) > 0 {
			alternativeServices[i].Name = prefix + "-" + alternativeServices[i].Name
		}
		if len(alternativeServices[i].Namespace) == 0 {
			alternativeServices[i].Namespace = namespace
		}
		if alternativeServices[i].Spec.Type == corev1.ServiceTypeLoadBalancer {
			alternativeServices[i].Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
		}
		if len(leaderName) > 0 {
			selector := alternativeServices[i].Spec.Selector
			if selector == nil {
				selector = make(map[string]string, 0)
			}
			selector[constant.RoleLabelKey] = leaderName
			alternativeServices[i].Spec.Selector = selector
		}
	}
	return alternativeServices
}

func getLeaderName(component *component.SynthesizedComponent) string {
	if component == nil {
		return ""
	}
	switch component.WorkloadType {
	case appsv1alpha1.Consensus:
		if component.ConsensusSpec != nil {
			return component.ConsensusSpec.Leader.Name
		}
	case appsv1alpha1.Replication:
		return constant.Primary
	}
	return ""
}

// separateServices separates 'services' to a main service from cd and alternative services from cluster
func separateServices(services []corev1.Service) (*corev1.Service, []corev1.Service) {
	if len(services) == 0 {
		return nil, nil
	}
	// from component.buildComponent (which contains component.Services' building process), the first item should be the main service
	// TODO(free6om): make two fields in component(i.e. Service and AlternativeServices) after RSM passes all testes.
	return &services[0], services[1:]
}

func buildRoleInfo(component *component.SynthesizedComponent) ([]workloads.ReplicaRole, *workloads.RoleProbe, *workloads.MembershipReconfiguration, *workloads.MemberUpdateStrategy) {
	var (
		roles           []workloads.ReplicaRole
		probe           *workloads.RoleProbe
		reconfiguration *workloads.MembershipReconfiguration
		strategy        *workloads.MemberUpdateStrategy
	)

	actions := buildActionFromCharacterType(component.CharacterType, component.WorkloadType == appsv1alpha1.Consensus)
	if actions != nil && component.Probes != nil && component.Probes.RoleProbe != nil {
		probe = &workloads.RoleProbe{ProbeActions: actions}
		roleProbe := component.Probes.RoleProbe
		probe.PeriodSeconds = roleProbe.PeriodSeconds
		probe.TimeoutSeconds = roleProbe.TimeoutSeconds
		probe.FailureThreshold = roleProbe.FailureThreshold
		// set to default value
		probe.SuccessThreshold = 1
	}

	// TODO(free6om): set default reconfiguration actions after relative addon refactored
	reconfiguration = nil

	switch component.WorkloadType {
	case appsv1alpha1.Consensus:
		roles, strategy = buildRoleInfoFromConsensus(component.ConsensusSpec)
	case appsv1alpha1.Replication:
		roles = buildRoleInfoFromReplication()
		reconfiguration = nil
		strgy := workloads.SerialUpdateStrategy
		strategy = &strgy
	}

	return roles, probe, reconfiguration, strategy
}

func buildRoleInfoFromReplication() []workloads.ReplicaRole {
	return []workloads.ReplicaRole{
		{
			Name:       constant.Primary,
			IsLeader:   true,
			CanVote:    true,
			AccessMode: workloads.ReadWriteMode,
		},
		{
			Name:       constant.Secondary,
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.ReadonlyMode,
		},
	}
}

func buildRoleInfoFromConsensus(consensusSpec *appsv1alpha1.ConsensusSetSpec) ([]workloads.ReplicaRole, *workloads.MemberUpdateStrategy) {
	if consensusSpec == nil {
		return nil, nil
	}

	var (
		roles    []workloads.ReplicaRole
		strategy *workloads.MemberUpdateStrategy
	)

	roles = append(roles, workloads.ReplicaRole{
		Name:       consensusSpec.Leader.Name,
		IsLeader:   true,
		CanVote:    true,
		AccessMode: workloads.AccessMode(consensusSpec.Leader.AccessMode),
	})
	for _, follower := range consensusSpec.Followers {
		roles = append(roles, workloads.ReplicaRole{
			Name:       follower.Name,
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.AccessMode(follower.AccessMode),
		})
	}
	if consensusSpec.Learner != nil {
		roles = append(roles, workloads.ReplicaRole{
			Name:       consensusSpec.Learner.Name,
			IsLeader:   false,
			CanVote:    false,
			AccessMode: workloads.AccessMode(consensusSpec.Learner.AccessMode),
		})
	}

	strgy := workloads.MemberUpdateStrategy(consensusSpec.UpdateStrategy)
	strategy = &strgy

	return roles, strategy
}

// buildActionFromCharacterType is a temporary workaround to provide vary engines' role probe functionality,
// as there is no way to configure these fields in Cluster API currently.
// TODO(free6om): remove this after ComponentDefinition API re-designed.
func buildActionFromCharacterType(characterType string, isConsensus bool) []workloads.Action {
	kind := strings.ToLower(characterType)
	switch kind {
	case "mysql": //nolint:goconst
		if isConsensus {
			return []workloads.Action{
				{
					Image: "arey/mysql-client:latest",
					Command: []string{
						"mysql",
						"-h127.0.0.1",
						"-P3306",
						"-u$KB_RSM_USERNAME",
						"-p$KB_RSM_PASSWORD",
						"-N",
						"-B",
						"-e",
						"\"select role from information_schema.wesql_cluster_local\"",
						"|",
						"xargs echo -n",
					},
				},
				{
					Command: []string{"echo $v_KB_RSM_LAST_STDOUT | tr '[:upper:]' '[:lower:]' | xargs echo -n"},
				},
			}
		}
		return []workloads.Action{
			{
				Image: "free6om/kubeblocks:latest",
				Command: []string{
					"curl http://127.0.0.1:3501/v1.0/bindings/mysql?operation=checkRole&workloadType=Replication",
				},
			},
			{
				Image: "jetbrainsinfra/jq:latest",
				Command: []string{
					"echo $v_KB_RSM_LAST_STDOUT | jq -r '.role' | tr '[:upper:]' '[:lower:]' | xargs echo -n",
				},
			},
		}
	case "postgres", "postgresql":
		return []workloads.Action{
			{
				Image: "governmentpaas/psql:latest",
				Command: []string{
					"PGPASSWORD=$KB_RSM_PASSWORD psql",
					"-h 127.0.0.1",
					"-p 5432",
					"-U $KB_RSM_USERNAME",
					"-w",
					"-t",
					"-c",
					"\"select pg_is_in_recovery();\"",
					"|",
					"xargs echo -n",
				},
			},
			{
				Command: []string{"if [ \"f\" = \"$v_KB_RSM_LAST_STDOUT\" ]; then echo -n \"primary\"; else echo -n \"secondary\"; fi"},
			},
		}
	case "mongodb":
		return []workloads.Action{
			{
				Image: "registry.cn-hangzhou.aliyuncs.com/apecloud/mongo:5.0.14",
				Command: []string{
					"Status=$(export CLIENT=`which mongosh>/dev/null&&echo mongosh||echo mongo`; $CLIENT -u $KB_RSM_USERNAME -p $KB_RSM_PASSWORD 127.0.0.1:27017 --quiet --eval \"JSON.stringify(rs.status())\") &&",
					"MyState=$(echo $Status | jq '.myState') &&",
					"echo $Status | jq \".members[] | select(.state == ($MyState | tonumber)) | .stateStr\" |tr '[:upper:]' '[:lower:]' | xargs echo -n",
				},
			},
		}
	case "etcd":
		return []workloads.Action{
			{
				Image: "quay.io/coreos/etcd:v3.5.6",
				Command: []string{
					"Status=$(etcdctl --endpoints=127.0.0.1:2379 endpoint status -w simple --command-timeout=300ms --dial-timeout=100m) &&",
					"IsLeader=$(echo $Status | awk -F ', ' '{print $5}') &&",
					"IsLearner=$(echo $Status | awk -F ', ' '{print $6}') &&",
					"if [ \"true\" = \"$IsLeader\" ]; then echo -n \"leader\"; elif [ \"true\" = \"$IsLearner\" ]; then echo -n \"learner\"; else echo -n \"follower\"; fi",
				},
			},
		}
	case "redis":
		return []workloads.Action{
			{
				Image: "registry.cn-hangzhou.aliyuncs.com/apecloud/redis-stack-server:7.0.6-RC8",
				Command: []string{
					"Role=$(redis-cli --user $KB_RSM_USERNAME --pass $KB_RSM_PASSWORD --no-auth-warning info | grep role | awk -F ':' '{print $2}' | tr '[:upper:]' '[:lower:]' | tr -d '\r' | tr -d '\n') &&",
					"if [ \"master\" = \"$Role\" ]; then echo -n \"primary\"; else echo -n \"secondary\"; fi",
				},
			},
		}
	}
	return nil
}

func randomString(length int) string {
	return rand.String(length)
}

func BuildConnCredential(clusterDefinition *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) *corev1.Secret {
	wellKnownLabels := buildWellKnownLabels(clusterDefinition.Name, cluster.Name, "")
	delete(wellKnownLabels, constant.KBAppComponentLabelKey)
	credentialBuilder := builder.NewSecretBuilder(cluster.Namespace, fmt.Sprintf("%s-conn-credential", cluster.Name)).
		AddLabelsInMap(wellKnownLabels).
		SetStringData(clusterDefinition.Spec.ConnectionCredential)
	if len(clusterDefinition.Spec.Type) > 0 {
		credentialBuilder.AddLabels("apps.kubeblocks.io/cluster-type", clusterDefinition.Spec.Type)
	}
	connCredential := credentialBuilder.GetObject()

	if len(connCredential.StringData) == 0 {
		return connCredential
	}

	replaceVarObjects := func(k, v *string, i int, origValue string, varObjectsMap map[string]string) {
		toReplace := origValue
		for j, r := range varObjectsMap {
			replaced := strings.ReplaceAll(toReplace, j, r)
			if replaced == toReplace {
				continue
			}
			toReplace = replaced
			// replace key
			if i == 0 {
				delete(connCredential.StringData, origValue)
				*k = replaced
			} else {
				*v = replaced
			}
		}
	}

	// REVIEW: perhaps handles value replacement at `func mergeComponents`
	replaceData := func(varObjectsMap map[string]string) {
		copyStringData := connCredential.DeepCopy().StringData
		for k, v := range copyStringData {
			for i, vv := range []string{k, v} {
				if !strings.Contains(vv, "$(") {
					continue
				}
				replaceVarObjects(&k, &v, i, vv, varObjectsMap)
			}
			connCredential.StringData[k] = v
		}
	}

	// TODO: do JIT value generation for lower CPU resources
	// 1st pass replace variables
	uuidVal := uuid.New()
	uuidBytes := uuidVal[:]
	uuidStr := uuidVal.String()
	uuidB64 := base64.RawStdEncoding.EncodeToString(uuidBytes)
	uuidStrB64 := base64.RawStdEncoding.EncodeToString([]byte(strings.ReplaceAll(uuidStr, "-", "")))
	uuidHex := hex.EncodeToString(uuidBytes)
	m := map[string]string{
		"$(RANDOM_PASSWD)":        randomString(8),
		"$(UUID)":                 uuidStr,
		"$(UUID_B64)":             uuidB64,
		"$(UUID_STR_B64)":         uuidStrB64,
		"$(UUID_HEX)":             uuidHex,
		"$(SVC_FQDN)":             fmt.Sprintf("%s-%s.%s.svc", cluster.Name, component.Name, cluster.Namespace),
		"$(KB_CLUSTER_COMP_NAME)": cluster.Name + "-" + component.Name,
		"$(HEADLESS_SVC_FQDN)":    fmt.Sprintf("%s-%s-headless.%s.svc", cluster.Name, component.Name, cluster.Namespace),
	}
	if len(component.Services) > 0 {
		for _, p := range component.Services[0].Spec.Ports {
			m[fmt.Sprintf("$(SVC_PORT_%s)", p.Name)] = strconv.Itoa(int(p.Port))
		}
	}
	replaceData(m)

	// 2nd pass replace $(CONN_CREDENTIAL) variables
	m = map[string]string{}
	for k, v := range connCredential.StringData {
		m[fmt.Sprintf("$(CONN_CREDENTIAL).%s", k)] = v
	}
	replaceData(m)
	return connCredential
}

func BuildPDB(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) *policyv1.PodDisruptionBudget {
	wellKnownLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	return builder.NewPDBBuilder(cluster.Namespace, fmt.Sprintf("%s-%s", cluster.Name, component.Name)).
		AddLabelsInMap(wellKnownLabels).
		AddLabels(constant.AppComponentLabelKey, component.CompDefName).
		AddSelectorsInMap(wellKnownLabels).
		GetObject()
}

func BuildDeploy(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent, envConfigName string) (*appsv1.Deployment, error) {
	const tplFile = "deployment_template.cue"
	deploy := appsv1.Deployment{}
	if err := buildFromCUE(tplFile, map[string]any{
		"cluster":   cluster,
		"component": component,
	}, "deployment", &deploy); err != nil {
		return nil, err
	}

	if component.StatelessSpec != nil {
		deploy.Spec.Strategy = component.StatelessSpec.UpdateStrategy
	}
	if err := processContainersInjection(reqCtx, cluster, component, envConfigName, &deploy.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return &deploy, nil
}

func BuildPVC(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	vct *corev1.PersistentVolumeClaimTemplate,
	pvcKey types.NamespacedName,
	snapshotName string) *corev1.PersistentVolumeClaim {
	wellKnownLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	pvcBuilder := builder.NewPVCBuilder(pvcKey.Namespace, pvcKey.Name).
		AddLabelsInMap(wellKnownLabels).
		AddLabels(constant.VolumeClaimTemplateNameLabelKey, vct.Name).
		SetAccessModes(vct.Spec.AccessModes).
		SetResources(vct.Spec.Resources)
	if vct.Spec.StorageClassName != nil {
		pvcBuilder.SetStorageClass(*vct.Spec.StorageClassName)
	}
	if len(snapshotName) > 0 {
		apiGroup := "snapshot.storage.k8s.io"
		pvcBuilder.SetDataSource(corev1.TypedLocalObjectReference{
			APIGroup: &apiGroup,
			Kind:     "VolumeSnapshot",
			Name:     snapshotName,
		})
	}
	pvc := pvcBuilder.GetObject()
	BuildPersistentVolumeClaimLabels(component, pvc, vct.Name)
	return pvc
}

// BuildEnvConfig builds cluster component context ConfigMap object, which is to be used in workload container's
// envFrom.configMapRef with name of "$(cluster.metadata.name)-$(component.name)-env" pattern.
func BuildEnvConfig(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) *corev1.ConfigMap {
	envData := map[string]string{}
	// add component envs
	if component.ComponentRefEnvs != nil {
		for _, env := range component.ComponentRefEnvs {
			envData[env.Name] = env.Value
		}
	}

	wellKnownLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	wellKnownLabels[constant.AppComponentLabelKey] = component.CompDefName
	return builder.NewConfigMapBuilder(cluster.Namespace, fmt.Sprintf("%s-%s-env", cluster.Name, component.Name)).
		AddLabelsInMap(wellKnownLabels).
		AddLabels(constant.AppConfigTypeLabelKey, "kubeblocks-env").
		SetData(envData).
		GetObject()
}

func BuildBackup(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	backupPolicyName string,
	backupKey types.NamespacedName,
	backupType string) *dataprotectionv1alpha1.Backup {
	return builder.NewBackupBuilder(backupKey.Namespace, backupKey.Name).
		AddLabels(constant.BackupTypeLabelKeyKey, backupType).
		AddLabels(constant.KBManagedByKey, "cluster").
		AddLabels("backuppolicies.dataprotection.kubeblocks.io/name", backupPolicyName).
		AddLabels(constant.AppNameLabelKey, component.ClusterDefName).
		AddLabels(constant.AppInstanceLabelKey, cluster.Name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		AddLabels(constant.KBAppComponentLabelKey, component.Name).
		SetBackupPolicyName(backupPolicyName).
		SetBackType(dataprotectionv1alpha1.BackupType(backupType)).
		GetObject()
}

func BuildConfigMapWithTemplate(cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	configs map[string]string,
	cmName string,
	configTemplateSpec appsv1alpha1.ComponentTemplateSpec) *corev1.ConfigMap {
	wellKnownLabels := buildWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)
	wellKnownLabels[constant.AppComponentLabelKey] = component.CompDefName
	return builder.NewConfigMapBuilder(cluster.Namespace, cmName).
		AddLabelsInMap(wellKnownLabels).
		AddLabels(constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType).
		AddLabels(constant.CMTemplateNameLabelKey, configTemplateSpec.TemplateRef).
		AddAnnotations(constant.DisableUpgradeInsConfigurationAnnotationKey, strconv.FormatBool(false)).
		SetData(configs).
		GetObject()
}

func BuildCfgManagerContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams, component *component.SynthesizedComponent) (*corev1.Container, error) {
	var env []corev1.EnvVar
	env = append(env, corev1.EnvVar{
		Name: "CONFIG_MANAGER_POD_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "status.podIP",
			},
		},
	})
	if len(sidecarRenderedParam.CharacterType) > 0 {
		env = append(env, corev1.EnvVar{
			Name:  "DB_TYPE",
			Value: sidecarRenderedParam.CharacterType,
		})
	}
	if sidecarRenderedParam.CharacterType == "mysql" {
		env = append(env, corev1.EnvVar{
			Name: "MYSQL_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  "username",
					LocalObjectReference: corev1.LocalObjectReference{Name: sidecarRenderedParam.SecreteName},
				},
			},
		},
			corev1.EnvVar{
				Name: "MYSQL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key:                  "password",
						LocalObjectReference: corev1.LocalObjectReference{Name: sidecarRenderedParam.SecreteName},
					},
				},
			},
			corev1.EnvVar{
				Name:  "DATA_SOURCE_NAME",
				Value: "$(MYSQL_USER):$(MYSQL_PASSWORD)@(localhost:3306)/",
			},
		)
	}
	containerBuilder := builder.NewContainerBuilder(sidecarRenderedParam.ManagerName).
		AddCommands("env").
		AddArgs("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$(TOOLS_PATH)").
		AddArgs("/bin/reloader").
		AddArgs(sidecarRenderedParam.Args...).
		AddEnv(env...).
		SetImage(sidecarRenderedParam.Image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddVolumeMounts(sidecarRenderedParam.Volumes...)
	if sidecarRenderedParam.ShareProcessNamespace {
		user := int64(0)
		containerBuilder.SetSecurityContext(corev1.SecurityContext{
			RunAsUser: &user,
		})
	}
	container := containerBuilder.GetObject()

	if err := injectEnvs(sidecarRenderedParam.Cluster, component, sidecarRenderedParam.EnvConfigName, container); err != nil {
		return nil, err
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	return container, nil
}

func BuildBackupManifestsJob(key types.NamespacedName, backup *dataprotectionv1alpha1.Backup, podSpec *corev1.PodSpec) *batchv1.Job {
	spec := podSpec.DeepCopy()
	spec.RestartPolicy = corev1.RestartPolicyNever
	ctx := spec.SecurityContext
	if ctx == nil {
		ctx = &corev1.PodSecurityContext{}
	}
	user := int64(0)
	ctx.RunAsUser = &user
	spec.SecurityContext = ctx
	return builder.NewJobBuilder(key.Namespace, key.Name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		SetPodTemplateSpec(corev1.PodTemplateSpec{Spec: *spec}).
		SetBackoffLimit(3).
		SetTTLSecondsAfterFinished(10).
		GetObject()
}

func BuildRestoreJob(cluster *appsv1alpha1.Cluster, synthesizedComponent *component.SynthesizedComponent, name, image string, command []string,
	volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, env []corev1.EnvVar, resources *corev1.ResourceRequirements) (*batchv1.Job, error) {
	containerBuilder := builder.NewContainerBuilder("restore").
		SetImage(image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddCommands(command...).
		AddVolumeMounts(volumeMounts...).
		AddEnv(env...)
	if resources != nil {
		containerBuilder.SetResources(*resources)
	}
	container := containerBuilder.GetObject()

	ctx := corev1.PodSecurityContext{}
	user := int64(0)
	ctx.RunAsUser = &user
	pod := builder.NewPodBuilder(cluster.Namespace, "").
		AddContainer(*container).
		AddVolumes(volumes...).
		SetRestartPolicy(corev1.RestartPolicyOnFailure).
		SetSecurityContext(ctx).
		GetObject()
	template := corev1.PodTemplateSpec{
		Spec: pod.Spec,
	}

	job := builder.NewJobBuilder(cluster.Namespace, name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		SetPodTemplateSpec(template).
		GetObject()
	containers := job.Spec.Template.Spec.Containers
	if len(containers) > 0 {
		if err := injectEnvs(cluster, synthesizedComponent, "", &containers[0]); err != nil {
			return nil, err
		}
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(&containers[0])
	}
	tolerations, err := component.BuildTolerations(cluster, cluster.Spec.GetComponentByName(synthesizedComponent.Name))
	if err != nil {
		return nil, err
	}
	job.Spec.Template.Spec.Tolerations = tolerations
	return job, nil
}

func BuildCfgManagerToolsContainer(sidecarRenderedParam *cfgcm.CfgManagerBuildParams, component *component.SynthesizedComponent, toolsMetas []appsv1alpha1.ToolConfig, toolsMap map[string]cfgcm.ConfigSpecMeta) ([]corev1.Container, error) {
	toolContainers := make([]corev1.Container, 0, len(toolsMetas))
	for _, toolConfig := range toolsMetas {
		toolContainerBuilder := builder.NewContainerBuilder(toolConfig.Name).
			AddCommands(toolConfig.Command...).
			SetImagePullPolicy(corev1.PullIfNotPresent).
			AddVolumeMounts(sidecarRenderedParam.Volumes...)
		if len(toolConfig.Image) > 0 {
			toolContainerBuilder.SetImage(toolConfig.Image)
		}
		toolContainers = append(toolContainers, *toolContainerBuilder.GetObject())
	}
	for i := range toolContainers {
		container := &toolContainers[i]
		if err := injectEnvs(sidecarRenderedParam.Cluster, component, sidecarRenderedParam.EnvConfigName, container); err != nil {
			return nil, err
		}
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
		if meta, ok := toolsMap[container.Name]; ok {
			setToolsScriptsPath(container, meta)
		}
	}
	return toolContainers, nil
}

func setToolsScriptsPath(container *corev1.Container, meta cfgcm.ConfigSpecMeta) {
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  cfgcm.KBTOOLSScriptsPathEnv,
		Value: filepath.Join(cfgcm.KBScriptVolumePath, meta.ConfigSpec.Name),
	})
}

func BuildVolumeSnapshotClass(name string, driver string) *snapshotv1.VolumeSnapshotClass {
	return builder.NewVolumeSnapshotClassBuilder("", name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		SetDriver(driver).
		SetDeletionPolicy(snapshotv1.VolumeSnapshotContentDelete).
		GetObject()
}

func BuildServiceAccount(cluster *appsv1alpha1.Cluster) *corev1.ServiceAccount {
	wellKnownLabels := buildWellKnownLabels(cluster.Spec.ClusterDefRef, cluster.Name, "")
	delete(wellKnownLabels, constant.KBAppComponentLabelKey)
	return builder.NewServiceAccountBuilder(cluster.Namespace, fmt.Sprintf("kb-%s", cluster.Name)).
		AddLabelsInMap(wellKnownLabels).
		GetObject()
}

func BuildRoleBinding(cluster *appsv1alpha1.Cluster) *rbacv1.RoleBinding {
	wellKnownLabels := buildWellKnownLabels(cluster.Spec.ClusterDefRef, cluster.Name, "")
	delete(wellKnownLabels, constant.KBAppComponentLabelKey)
	return builder.NewRoleBindingBuilder(cluster.Namespace, fmt.Sprintf("kb-%s", cluster.Name)).
		AddLabelsInMap(wellKnownLabels).
		SetRoleRef(rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.RBACRoleName,
		}).
		AddSubjects(rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("kb-%s", cluster.Name),
		}).
		GetObject()
}

func BuildClusterRoleBinding(cluster *appsv1alpha1.Cluster) *rbacv1.ClusterRoleBinding {
	wellKnownLabels := buildWellKnownLabels(cluster.Spec.ClusterDefRef, cluster.Name, "")
	delete(wellKnownLabels, constant.KBAppComponentLabelKey)
	return builder.NewClusterRoleBindingBuilder(cluster.Namespace, fmt.Sprintf("kb-%s", cluster.Name)).
		AddLabelsInMap(wellKnownLabels).
		SetRoleRef(rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     constant.RBACClusterRoleName,
		}).
		AddSubjects(rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("kb-%s", cluster.Name),
		}).
		GetObject()
}
