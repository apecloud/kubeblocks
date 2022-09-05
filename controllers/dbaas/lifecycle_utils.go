/*
Copyright 2022.

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

package dbaas

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/leaanthony/debme"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type createParams struct {
	clusterDefinition *dbaasv1alpha1.ClusterDefinition
	cluster           *dbaasv1alpha1.Cluster
	component         *Component
	roleGroup         *RoleGroup
	applyObjs         *[]client.Object
	cacheCtx          *map[string]interface{}
}

const (
	dbaasPrefix = "OPENDBAAS"
)

var (
	//go:embed cue/*
	cueTemplates embed.FS
)

func (c createParams) getCacheBytesValue(key string, valueCreator func() ([]byte, error)) ([]byte, error) {
	vIf, ok := (*c.cacheCtx)[key]
	if ok {
		return vIf.([]byte), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	(*c.cacheCtx)[key] = v
	return v, err
}

func (c createParams) getCacheCUETplValue(key string, valueCreator func() (*intctrlutil.CUETpl, error)) (*intctrlutil.CUETpl, error) {
	vIf, ok := (*c.cacheCtx)[key]
	if ok {
		return vIf.(*intctrlutil.CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	(*c.cacheCtx)[key] = v
	return v, err
}

func getClusterDefinitionComponentByType(components []dbaasv1alpha1.ClusterDefinitionComponent, typeName string) *dbaasv1alpha1.ClusterDefinitionComponent {
	for _, component := range components {
		if component.TypeName == typeName {
			return &component
		}
	}
	return nil
}

func getAppVersionComponentByType(components []dbaasv1alpha1.AppVersionComponent, typeName string) *dbaasv1alpha1.AppVersionComponent {
	for _, component := range components {
		if component.Type == typeName {
			return &component
		}
	}
	return nil
}

func getClusterComponentsByType(components []dbaasv1alpha1.ClusterComponent, typeName string) []*dbaasv1alpha1.ClusterComponent {
	comps := []*dbaasv1alpha1.ClusterComponent{}
	for _, component := range components {
		if component.Type == typeName {
			comps = append(comps, &component)
		}
	}
	return comps
}

func getRoleGroupTemplateByType(roleGroups []dbaasv1alpha1.RoleGroupTemplate, typeName string) *dbaasv1alpha1.RoleGroupTemplate {
	for _, roleGroup := range roleGroups {
		if roleGroup.TypeName == typeName {
			return &roleGroup
		}
	}
	return nil
}

func getClusterRoleGroupByType(clusterRoleGroups []dbaasv1alpha1.ClusterRoleGroup, typeName string) *dbaasv1alpha1.ClusterRoleGroup {
	for _, roleGroup := range clusterRoleGroups {
		if roleGroup.Type == typeName {
			return &roleGroup
		}
	}
	return nil
}

func getContainerByName(containers []corev1.Container, name string) (int, *corev1.Container) {
	for i, container := range containers {
		if container.Name == name {
			return i, &container
		}
	}
	return -1, nil
}

func toK8sVolumeClaimTemplate(template dbaasv1alpha1.ClusterComponentVolumeClaimTemplate) corev1.PersistentVolumeClaimTemplate {
	t := corev1.PersistentVolumeClaimTemplate{}
	t.ObjectMeta.Name = template.Name
	t.Spec = template.Spec
	return t
}

func toK8sVolumeClaimTemplates(templates []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaimTemplate {
	ts := []corev1.PersistentVolumeClaimTemplate{}
	for _, template := range templates {
		ts = append(ts, toK8sVolumeClaimTemplate(template))
	}
	return ts
}

func mergeComponents(
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent,
	appVerComp *dbaasv1alpha1.AppVersionComponent,
	clusterComp *dbaasv1alpha1.ClusterComponent) *Component {
	if clusterDefComp == nil {
		return nil
	}
	component := &Component{
		ClusterDefName:  clusterDef.Name,
		ClusterType:     clusterDef.Spec.Type,
		Name:            clusterDefComp.TypeName,
		Type:            clusterDefComp.TypeName,
		RoleGroupNames:  clusterDefComp.RoleGroups,
		MinAvailable:    clusterDefComp.MinAvailable,
		MaxAvailable:    clusterDefComp.MaxAvailable,
		DefaultReplicas: clusterDefComp.DefaultReplicas,
		IsStateless:     clusterDefComp.IsStateless,
		AntiAffinity:    clusterDefComp.AntiAffinity,
		IsQuorum:        clusterDefComp.IsQuorum,
		Strategies:      clusterDefComp.Strategies,
		Containers:      clusterDefComp.Containers,
		Service:         clusterDefComp.Service,
		Scripts:         clusterDefComp.Scripts,
	}
	if clusterComp != nil {
		component.Name = clusterComp.Name
	}

	if appVerComp != nil && appVerComp.Containers != nil {
		for _, container := range appVerComp.Containers {
			i, c := getContainerByName(component.Containers, container.Name)
			if c != nil {
				if container.Image != "" {
					component.Containers[i].Image = container.Image
				}
				if len(container.Command) != 0 {
					component.Containers[i].Command = container.Command
				}
				if len(container.Args) != 0 {
					component.Containers[i].Args = container.Args
				}
				if container.WorkingDir != "" {
					component.Containers[i].WorkingDir = container.WorkingDir
				}
				if len(container.Ports) != 0 {
					component.Containers[i].Ports = container.Ports
				}
				if len(container.EnvFrom) != 0 {
					component.Containers[i].EnvFrom = container.EnvFrom
				}
				if len(container.Env) != 0 {
					component.Containers[i].Env = container.Env
				}
				if container.Resources.Limits != nil || container.Resources.Requests != nil {
					component.Containers[i].Resources = container.Resources
				}
				if len(container.VolumeMounts) != 0 {
					component.Containers[i].VolumeMounts = container.VolumeMounts
				}
				if len(container.VolumeDevices) != 0 {
					component.Containers[i].VolumeDevices = container.VolumeDevices
				}
				if container.LivenessProbe != nil {
					component.Containers[i].LivenessProbe = container.LivenessProbe
				}
				if container.ReadinessProbe != nil {
					component.Containers[i].ReadinessProbe = container.ReadinessProbe
				}
				if container.StartupProbe != nil {
					component.Containers[i].StartupProbe = container.StartupProbe
				}
				if container.Lifecycle != nil {
					component.Containers[i].Lifecycle = container.Lifecycle
				}
				if container.TerminationMessagePath != "" {
					component.Containers[i].TerminationMessagePath = container.TerminationMessagePath
				}
				if container.TerminationMessagePolicy != "" {
					component.Containers[i].TerminationMessagePolicy = container.TerminationMessagePolicy
				}
				if container.ImagePullPolicy != "" {
					component.Containers[i].ImagePullPolicy = container.ImagePullPolicy
				}
				if container.SecurityContext != nil {
					component.Containers[i].SecurityContext = container.SecurityContext
				}
			} else {
				component.Containers = append(component.Containers, container)
			}
		}
	}
	if clusterComp != nil {
		component.Name = clusterComp.Name
		if clusterComp.VolumeClaimTemplates != nil {
			component.VolumeClaimTemplates = toK8sVolumeClaimTemplates(clusterComp.VolumeClaimTemplates)
		}
		if clusterComp.Resources.Requests != nil || clusterComp.Resources.Limits != nil {
			component.Containers[0].Resources = clusterComp.Resources
		}
		component.RoleGroups = clusterComp.RoleGroups
	}
	if component.VolumeClaimTemplates == nil {
		for i := range component.Containers {
			component.Containers[i].VolumeMounts = nil
		}
	}
	return component
}

func mergeRoleGroups(roleGroupTemplate *dbaasv1alpha1.RoleGroupTemplate, clusterRoleGroup *dbaasv1alpha1.ClusterRoleGroup) *RoleGroup {
	if roleGroupTemplate == nil {
		return nil
	}
	roleGroup := &RoleGroup{}
	roleGroup.Type = roleGroupTemplate.TypeName
	roleGroup.Scripts = roleGroupTemplate.Scripts
	roleGroup.Replicas = roleGroupTemplate.DefaultReplicas
	roleGroup.MaxAvailable = roleGroupTemplate.MaxAvailable
	roleGroup.MinAvailable = roleGroupTemplate.MinAvailable
	roleGroup.UpdateStrategy = roleGroupTemplate.UpdateStrategy
	roleGroup.Name = roleGroupTemplate.TypeName
	if clusterRoleGroup == nil || clusterRoleGroup.Type != roleGroupTemplate.TypeName {
		return roleGroup
	}
	roleGroup.Name = clusterRoleGroup.Name
	if clusterRoleGroup.Replicas >= 0 {
		roleGroup.Replicas = clusterRoleGroup.Replicas
	}
	roleGroup.Service = clusterRoleGroup.Service
	return roleGroup
}

func buildClusterCreationTasks(
	clusterDefinition *dbaasv1alpha1.ClusterDefinition,
	appVersion *dbaasv1alpha1.AppVersion,
	cluster *dbaasv1alpha1.Cluster) (*intctrlutil.Task, error) {
	rootTask := intctrlutil.NewTask()

	orderedComponentNames := clusterDefinition.Spec.Cluster.Strategies.Create.Order
	components := clusterDefinition.Spec.Components

	if len(orderedComponentNames) == 0 {
		for _, comp := range clusterDefinition.Spec.Components {
			orderedComponentNames = append(orderedComponentNames, comp.TypeName)
		}
	}

	applyObjs := make([]client.Object, 0, 3)
	cacheCtx := map[string]interface{}{}

	prepareSecretsTask := intctrlutil.NewTask()
	prepareSecretsTask.ExecFunction = prepareSecretObjs
	params := createParams{
		cluster:           cluster,
		clusterDefinition: clusterDefinition,
		applyObjs:         &applyObjs,
		cacheCtx:          &cacheCtx,
	}
	prepareSecretsTask.Context["exec"] = &params
	rootTask.SubTasks = append(rootTask.SubTasks, prepareSecretsTask)

	roleGroups := clusterDefinition.Spec.RoleGroupTemplates
	buildTask := func(component *Component, orderedRoleGroupNames []string) {
		componentTask := intctrlutil.NewTask()
		for _, roleGroupName := range orderedRoleGroupNames {
			roleGroupTemplate := getRoleGroupTemplateByType(roleGroups, roleGroupName)
			clusterRoleGroup := getClusterRoleGroupByType(component.RoleGroups, roleGroupName)
			roleGroup := mergeRoleGroups(roleGroupTemplate, clusterRoleGroup)
			roleGroupTask := intctrlutil.NewTask()
			roleGroupTask.ExecFunction = prepareRoleGroupObjs
			iParams := params
			iParams.component = component
			iParams.roleGroup = roleGroup
			roleGroupTask.Context["exec"] = &iParams
			componentTask.SubTasks = append(componentTask.SubTasks, roleGroupTask)
		}
		rootTask.SubTasks = append(rootTask.SubTasks, componentTask)
	}

	useDefaultComp := len(cluster.Spec.Components) == 0
	for _, componentName := range orderedComponentNames {
		clusterDefComponent := getClusterDefinitionComponentByType(components, componentName)
		orderedRoleGroupNames := clusterDefComponent.Strategies.Create.Order
		appVersionComponent := getAppVersionComponentByType(appVersion.Spec.Components, componentName)
		if len(orderedRoleGroupNames) == 0 {
			orderedRoleGroupNames = clusterDefComponent.RoleGroups
		}

		if useDefaultComp {
			buildTask(mergeComponents(clusterDefinition, clusterDefComponent, appVersionComponent, nil), orderedRoleGroupNames)
		} else {
			clusterComps := getClusterComponentsByType(cluster.Spec.Components, componentName)
			for _, clusterComp := range clusterComps {
				buildTask(mergeComponents(clusterDefinition, clusterDefComponent, appVersionComponent, clusterComp), orderedRoleGroupNames)
			}
		}
	}

	createObjsTask := intctrlutil.NewTask()
	createObjsTask.ExecFunction = checkedCreateObjs
	createObjsTask.Context["exec"] = &params
	rootTask.SubTasks = append(rootTask.SubTasks, createObjsTask)
	return &rootTask, nil
}

func checkedCreateObjs(ctx context.Context, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	if err := createOrReplaceResources(ctx, cli, params.cluster, *params.applyObjs); err != nil {
		return err
	}
	return nil
}

func prepareSecretObjs(ctx context.Context, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	secret, err := buildSecret(*params)
	if err != nil {
		return err
	}
	// must make sure secret resources are created before others
	*params.applyObjs = append(*params.applyObjs, secret)
	return nil
}

func prepareRoleGroupObjs(ctx context.Context, cli client.Client, obj interface{}) error {
	params, ok := obj.(*createParams)
	if !ok {
		return fmt.Errorf("invalid arg")
	}

	if params.component.IsStateless {
		sts, err := buildDeploy(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, sts)
	} else {
		sts, err := buildSts(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, sts)

		svcs, err := buildSvcs(*params, sts)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svcs...)
	}

	pdb, err := buildPdb(*params)
	if err != nil {
		return err
	}
	*params.applyObjs = append(*params.applyObjs, pdb)

	if params.roleGroup.Service.Ports != nil {
		svc, err := buildSvc(*params)
		if err != nil {
			return err
		}
		*params.applyObjs = append(*params.applyObjs, svc)
	}

	return nil
}

func createOrReplaceResources(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	objs []client.Object) error {
	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	for _, obj := range objs {
		if err := controllerutil.SetOwnerReference(cluster, obj, scheme); err != nil {
			return err
		}
		if err := cli.Create(ctx, obj); err == nil {
			continue
		} else if !apierrors.IsAlreadyExists(err) {
			return err
		}

		if !controllerutil.ContainsFinalizer(obj, dbClusterFinalizerName) {
			controllerutil.AddFinalizer(obj, dbClusterFinalizerName)
		}

		// Secret kind objects should only be applied once
		_, ok := obj.(*corev1.Secret)
		if ok {
			continue
		}
		// -

		key := client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}
		stsProto, ok := obj.(*appsv1.StatefulSet)
		if ok {
			stsObj := &appsv1.StatefulSet{}
			if err := cli.Get(ctx, key, stsObj); err != nil {
				return err
			}
			stsObj.Spec.Template = stsProto.Spec.Template
			stsObj.Spec.Replicas = stsProto.Spec.Replicas
			stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
			if err := cli.Update(ctx, stsObj); err != nil {
				return err
			}
			// check stsObj.Spec.VolumeClaimTemplates storage
			// request size and find attached PVC and patch request
			// storage size
			for _, vct := range stsObj.Spec.VolumeClaimTemplates {
				var vctProto *corev1.PersistentVolumeClaim
				for _, i := range stsProto.Spec.VolumeClaimTemplates {
					if i.Name == stsProto.Name {
						vctProto = &i
						break
					}
				}

				// REVIEW: how could VCT proto is nil?
				if vctProto == nil {
					continue
				}

				for i := *stsObj.Spec.Replicas - 1; i >= 0; i-- {
					pvc := &corev1.PersistentVolumeClaim{}
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					if err := cli.Get(ctx, pvcKey, pvc); err != nil {
						return err
					}
					if pvc.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Spec.Resources.Requests[corev1.ResourceStorage] {
						continue
					}
					patch := client.MergeFrom(pvc.DeepCopy())
					pvc.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
					if err := cli.Patch(ctx, pvc, patch); err != nil {
						return err
					}
				}
			}
			continue
		}
		deployProto, ok := obj.(*appsv1.Deployment)
		if ok {
			deployObj := &appsv1.Deployment{}
			if err := cli.Get(ctx, key, deployObj); err != nil {
				return err
			}
			deployObj.Spec = deployProto.Spec
			if err := cli.Update(ctx, deployObj); err != nil {
				return err
			}
			continue
		}
		svcProto, ok := obj.(*corev1.Service)
		if ok {
			svcObj := &corev1.Service{}
			if err := cli.Get(ctx, key, svcObj); err != nil {
				return err
			}
			svcObj.Spec = svcProto.Spec
			if err := cli.Update(ctx, svcObj); err != nil {
				return err
			}
			continue
		}
	}
	return nil
}

func buildSvcs(params createParams, sts *appsv1.StatefulSet) ([]client.Object, error) {
	stsPodLabels := sts.Spec.Template.Labels
	replicas := *sts.Spec.Replicas
	svcs := make([]client.Object, replicas)
	for i := 0; i < int(replicas); i++ {
		pod := &corev1.Pod{}
		pod.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.GetName(), i)
		pod.ObjectMeta.Namespace = sts.Namespace
		pod.ObjectMeta.Labels = map[string]string{
			"statefulset.kubernetes.io/pod-name": pod.ObjectMeta.Name,
			"app.kubernetes.io/name":             stsPodLabels["app.kubernetes.io/name"],
			"app.kubernetes.io/instance":         stsPodLabels["app.kubernetes.io/instance"],
			"app.kubernetes.io/component":        stsPodLabels["app.kubernetes.io/name"],
		}
		pod.Spec.Containers = sts.Spec.Template.Spec.Containers

		svc, err := buildHeadlessService(params, pod)
		if err != nil {
			return nil, err
		}
		svcs[i] = svc
	}
	return svcs, nil
}

func buildSvc(params createParams) (*corev1.Service, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("service_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("service_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	roleGroupStrByte, err := json.Marshal(params.roleGroup)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("roleGroup", roleGroupStrByte); err != nil {
		return nil, err
	}

	svcStrByte, err := cueValue.Lookup("service")
	if err != nil {
		return nil, err
	}

	svc := corev1.Service{}
	if err = json.Unmarshal(svcStrByte, &svc); err != nil {
		return nil, err
	}

	return &svc, nil
}

func randomString(length int) string {
	res, _ := password.Generate(length, 0, 0, false, false)
	return res
}

func buildSecret(params createParams) (*corev1.Secret, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("secret_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("secret_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterDefinitionStrByte, err := params.getCacheBytesValue("clusterDefinition", func() ([]byte, error) {
		return json.Marshal(params.clusterDefinition)
	})
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("clusterdefinition", clusterDefinitionStrByte); err != nil {
		return nil, err
	}

	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	if err = cueValue.FillRaw("secret.stringData.password", randomString(8)); err != nil {
		return nil, err
	}

	secretStrByte, err := cueValue.Lookup("secret")
	if err != nil {
		return nil, err
	}

	secret := corev1.Secret{}
	if err = json.Unmarshal(secretStrByte, &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

func buildSts(params createParams) (*appsv1.StatefulSet, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("statefulset_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("statefulset_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	roleGroupStrByte, err := json.Marshal(params.roleGroup)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("roleGroup", roleGroupStrByte); err != nil {
		return nil, err
	}

	stsStrByte, err := cueValue.Lookup("statefulset")
	if err != nil {
		return nil, err
	}

	sts := appsv1.StatefulSet{}
	if err = json.Unmarshal(stsStrByte, &sts); err != nil {
		return nil, err
	}

	stsStrByte = injectEnv(stsStrByte, dbaasPrefix+"_MY_SECRET_NAME", params.cluster.Name)

	if err = json.Unmarshal(stsStrByte, &sts); err != nil {
		return nil, err
	}

	prefix := dbaasPrefix + "_" + strings.ToUpper(params.component.Type) + "_" + strings.ToUpper(params.roleGroup.Name) + "_"
	replicas := int(*sts.Spec.Replicas)
	for i := range sts.Spec.Template.Spec.Containers {
		// inject self scope env
		c := &sts.Spec.Template.Spec.Containers[i]
		c.Env = append(c.Env, corev1.EnvVar{
			Name: dbaasPrefix + "_MY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		})
		// inject roleGroup scope env
		c.Env = append(c.Env, corev1.EnvVar{
			Name:      prefix + "N",
			Value:     strconv.Itoa(replicas),
			ValueFrom: nil,
		})
		for j := 0; j < replicas; j++ {
			c.Env = append(c.Env, corev1.EnvVar{
				Name:      prefix + strconv.Itoa(j) + "_HOSTNAME",
				Value:     sts.Name + "-" + strconv.Itoa(j),
				ValueFrom: nil,
			})
		}
	}
	return &sts, nil
}

func buildDeploy(params createParams) (*appsv1.Deployment, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("deployment_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("deployment_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	roleGroupStrByte, err := json.Marshal(params.roleGroup)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("roleGroup", roleGroupStrByte); err != nil {
		return nil, err
	}

	stsStrByte, err := cueValue.Lookup("deployment")
	if err != nil {
		return nil, err
	}

	deploy := appsv1.Deployment{}
	if err = json.Unmarshal(stsStrByte, &deploy); err != nil {
		return nil, err
	}

	stsStrByte = injectEnv(stsStrByte, dbaasPrefix+"_MY_SECRET_NAME", params.cluster.Name)

	if err = json.Unmarshal(stsStrByte, &deploy); err != nil {
		return nil, err
	}

	// TODO: inject environment

	return &deploy, nil
}

func buildHeadlessService(params createParams, pod *corev1.Pod) (*corev1.Service, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("headless_service_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("headless_service_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	podStrByte, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("pod", podStrByte); err != nil {
		return nil, err
	}

	svcStrByte, err := cueValue.Lookup("service")
	if err != nil {
		return nil, err
	}
	svc := corev1.Service{}
	if err = json.Unmarshal(svcStrByte, &svc); err != nil {
		return nil, err
	}

	scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
	if err = controllerutil.SetOwnerReference(params.cluster, &svc, scheme); err != nil {
		return nil, err
	}

	return &svc, nil
}

func buildPdb(params createParams) (*policyv1.PodDisruptionBudget, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("pdb_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("pdb_template.cue"))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	clusterStrByte, err := params.getCacheBytesValue("cluster", func() ([]byte, error) {
		return json.Marshal(params.cluster)
	})
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("cluster", clusterStrByte); err != nil {
		return nil, err
	}

	componentStrByte, err := json.Marshal(params.component)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("component", componentStrByte); err != nil {
		return nil, err
	}

	roleGroupStrByte, err := json.Marshal(params.roleGroup)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("roleGroup", roleGroupStrByte); err != nil {
		return nil, err
	}

	pdbStrByte, err := cueValue.Lookup("pdb")
	if err != nil {
		return nil, err
	}

	pdb := policyv1.PodDisruptionBudget{}
	if err = json.Unmarshal(pdbStrByte, &pdb); err != nil {
		return nil, err
	}

	return &pdb, nil
}

func injectEnv(strByte []byte, key string, value string) []byte {
	str := string(strByte)
	str = strings.ReplaceAll(str, "$("+key+")", value)
	return []byte(str)
}
