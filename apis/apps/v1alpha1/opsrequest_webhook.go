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

package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	KBSwitchoverCandidateInstanceForAnyPod = "*"
)

// log is for logging in this package.
var (
	opsRequestLog           = logf.Log.WithName("opsrequest-resource")
	opsRequestAnnotationKey = "kubeblocks.io/ops-request"
	// OpsRequestBehaviourMapper records the opsRequest behaviour according to the OpsType.
	OpsRequestBehaviourMapper = map[OpsType]OpsRequestBehaviour{}
)

func (r *OpsRequest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-opsrequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=vopsrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpsRequest{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateCreate() (admission.Warnings, error) {
	opsRequestLog.Info("validate create", "name", r.Name)
	return nil, r.validateEntry(true)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	opsRequestLog.Info("validate update", "name", r.Name)
	lastOpsRequest := old.(*OpsRequest).DeepCopy()
	// if no spec updated, we should skip validation.
	// if not, we can not delete the OpsRequest when cluster has been deleted.
	// because when cluster not existed, r.validate will report an error.
	if reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) {
		return nil, nil
	}

	if r.IsComplete() {
		return nil, fmt.Errorf("update OpsRequest: %s is forbidden when status.Phase is %s", r.Name, r.Status.Phase)
	}

	// Keep the cancel consistent between the two opsRequest for comparing the diff.
	lastOpsRequest.Spec.Cancel = r.Spec.Cancel
	if !reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) && r.Status.Phase != "" {
		return nil, fmt.Errorf("update OpsRequest: %s is forbidden except for cancel when status.Phase is %s", r.Name, r.Status.Phase)
	}
	return nil, r.validateEntry(false)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateDelete() (admission.Warnings, error) {
	opsRequestLog.Info("validate delete", "name", r.Name)
	return nil, nil
}

// IsComplete checks if opsRequest has been completed.
func (r *OpsRequest) IsComplete(phases ...OpsPhase) bool {
	completedPhase := func(phase OpsPhase) bool {
		return slices.Contains([]OpsPhase{OpsCancelledPhase, OpsSucceedPhase, OpsAbortedPhase, OpsFailedPhase}, phase)
	}
	if len(phases) == 0 {
		return completedPhase(r.Status.Phase)
	}
	for i := range phases {
		if !completedPhase(phases[i]) {
			return false
		}
	}
	return true
}

// Force checks if the current opsRequest can be forcibly executed
func (r *OpsRequest) Force() bool {
	// ops of type 'Start' do not support force execution.
	return r.Spec.Force && r.Spec.Type != StartType
}

// validateClusterPhase validates whether the current cluster state supports the OpsRequest
func (r *OpsRequest) validateClusterPhase(cluster *Cluster) error {
	opsBehaviour := OpsRequestBehaviourMapper[r.Spec.Type]
	// if the OpsType has no cluster phases, ignore it
	if len(opsBehaviour.FromClusterPhases) == 0 {
		return nil
	}
	if r.Force() {
		return nil
	}
	// validate whether existing the same type OpsRequest
	var (
		opsRequestValue string
		opsRecorder     []OpsRecorder
		ok              bool
	)
	if opsRequestValue, ok = cluster.Annotations[opsRequestAnnotationKey]; ok {
		// opsRequest annotation value in cluster to map
		if err := json.Unmarshal([]byte(opsRequestValue), &opsRecorder); err != nil {
			return err
		}
	}
	// check if the opsRequest can be executed in the current cluster.
	if slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		return nil
	}
	// check if this opsRequest needs to verify cluster phase before opsRequest starts running.
	needCheck := len(opsRecorder) == 0 || (opsRecorder[0].Name == r.Name && opsRecorder[0].InQueue)
	if !needCheck {
		return nil
	}
	// if TTLSecondsBeforeAbort is not set or 0, return error
	if r.Spec.PreConditionDeadlineSeconds == nil || *r.Spec.PreConditionDeadlineSeconds == 0 {
		return fmt.Errorf("OpsRequest.spec.type=%s is forbidden when Cluster.status.phase=%s", r.Spec.Type, cluster.Status.Phase)
	}
	return nil
}

// getCluster gets cluster with webhook client
func (r *OpsRequest) getCluster(ctx context.Context, k8sClient client.Client) (*Cluster, error) {
	if k8sClient == nil {
		return nil, nil
	}
	cluster := &Cluster{}
	// get cluster resource
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: r.Spec.GetClusterName()}, cluster); err != nil {
		return nil, fmt.Errorf("get cluster: %s failed, err: %s", r.Spec.GetClusterName(), err.Error())
	}
	return cluster, nil
}

func (r *OpsRequest) getConfigMap(ctx context.Context,
	k8sClient client.Client,
	cmName string) (*corev1.ConfigMap, error) {
	cmObj := &corev1.ConfigMap{}
	cmKey := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      cmName,
	}

	if err := k8sClient.Get(ctx, cmKey, cmObj); err != nil {
		return nil, err
	}
	return cmObj, nil
}

// Validate validates OpsRequest
func (r *OpsRequest) Validate(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster,
	needCheckClusterPhase bool) error {
	if needCheckClusterPhase {
		if err := r.validateClusterPhase(cluster); err != nil {
			return err
		}
	}
	return r.validateOps(ctx, k8sClient, cluster)
}

// ValidateEntry OpsRequest webhook validate entry
func (r *OpsRequest) validateEntry(isCreate bool) error {
	if webhookMgr == nil || webhookMgr.client == nil {
		return nil
	}
	ctx := context.Background()
	k8sClient := webhookMgr.client
	cluster, err := r.getCluster(ctx, k8sClient)
	if err != nil {
		return err
	}
	return r.Validate(ctx, k8sClient, cluster, isCreate)
}

// validateOps validates ops attributes
func (r *OpsRequest) validateOps(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster) error {
	// Check whether the corresponding attribute is legal according to the operation type
	switch r.Spec.Type {
	case UpgradeType:
		return r.validateUpgrade(ctx, k8sClient, cluster)
	case VerticalScalingType:
		return r.validateVerticalScaling(cluster)
	case HorizontalScalingType:
		return r.validateHorizontalScaling(ctx, k8sClient, cluster)
	case VolumeExpansionType:
		return r.validateVolumeExpansion(ctx, k8sClient, cluster)
	case RestartType:
		return r.validateRestart(cluster)
	case ReconfiguringType:
		return r.validateReconfigure(ctx, k8sClient, cluster)
	case SwitchoverType:
		return r.validateSwitchover(ctx, k8sClient, cluster)
	case DataScriptType:
		return r.validateDataScript(ctx, k8sClient, cluster)
	case ExposeType:
		return r.validateExpose(ctx, cluster)
	case RebuildInstanceType:
		return r.validateRebuildInstance(cluster)
	}
	return nil
}

// validateExpose validates expose api when spec.type is Expose
func (r *OpsRequest) validateExpose(_ context.Context, cluster *Cluster) error {
	exposeList := r.Spec.ExposeList
	if exposeList == nil {
		return notEmptyError("spec.expose")
	}

	var compOpsList []ComponentOps
	counter := 0
	for _, v := range exposeList {
		if len(v.ComponentName) > 0 {
			compOpsList = append(compOpsList, ComponentOps{ComponentName: v.ComponentName})
			continue
		} else {
			counter++
		}
		if counter > 1 {
			return fmt.Errorf("at most one spec.expose.componentName can be empty")
		}
		if v.Switch == EnableExposeSwitch {
			for _, opssvc := range v.Services {
				if len(opssvc.Ports) == 0 {
					return fmt.Errorf("spec.expose.services.ports must be specified when componentName is empty")
				}
			}
		}
	}
	return r.checkComponentExistence(cluster, compOpsList)
}

func (r *OpsRequest) validateRebuildInstance(cluster *Cluster) error {
	rebuildFrom := r.Spec.RebuildFrom
	if len(rebuildFrom) == 0 {
		return notEmptyError("spec.rebuildFrom")
	}
	var compOpsList []ComponentOps
	for _, v := range rebuildFrom {
		compOpsList = append(compOpsList, v.ComponentOps)
	}
	return r.checkComponentExistence(cluster, compOpsList)
}

// validateUpgrade validates spec.restart
func (r *OpsRequest) validateRestart(cluster *Cluster) error {
	restartList := r.Spec.RestartList
	if len(restartList) == 0 {
		return notEmptyError("spec.restart")
	}
	return r.checkComponentExistence(cluster, restartList)
}

// validateUpgrade validates spec.clusterOps.upgrade
func (r *OpsRequest) validateUpgrade(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster) error {
	upgrade := r.Spec.Upgrade
	if upgrade == nil {
		return notEmptyError("spec.upgrade")
	}
	if upgrade.ClusterVersionRef != nil && *upgrade.ClusterVersionRef != "" {
		// TODO: remove this deprecated api after v0.9
		return k8sClient.Get(ctx, types.NamespacedName{Name: *upgrade.ClusterVersionRef}, &ClusterVersion{})
	}
	if len(r.Spec.Upgrade.Components) == 0 {
		return notEmptyError("spec.upgrade.components")
	}
	return nil
}

// validateVerticalScaling validates api when spec.type is VerticalScaling
func (r *OpsRequest) validateVerticalScaling(cluster *Cluster) error {
	verticalScalingList := r.Spec.VerticalScalingList
	if len(verticalScalingList) == 0 {
		return notEmptyError("spec.verticalScaling")
	}

	// validate resources is legal and get component name slice
	compOpsList := make([]ComponentOps, len(verticalScalingList))
	for i, v := range verticalScalingList {
		compOpsList[i] = v.ComponentOps
		var instanceNames []string
		for j := range v.Instances {
			instanceNames = append(instanceNames, v.Instances[j].Name)
		}
		if err := r.checkInstanceTemplate(cluster, v.ComponentOps, instanceNames); err != nil {
			return err
		}
		if invalidValue, err := validateVerticalResourceList(v.Requests); err != nil {
			return invalidValueError(invalidValue, err.Error())
		}
		if invalidValue, err := validateVerticalResourceList(v.Limits); err != nil {
			return invalidValueError(invalidValue, err.Error())
		}
		if invalidValue, err := compareRequestsAndLimits(v.ResourceRequirements); err != nil {
			return invalidValueError(invalidValue, err.Error())
		}
	}
	return r.checkComponentExistence(cluster, compOpsList)
}

// validateVerticalScaling validate api is legal when spec.type is VerticalScaling
func (r *OpsRequest) validateReconfigure(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster) error {
	reconfigure := r.Spec.Reconfigure
	if reconfigure == nil && len(r.Spec.Reconfigures) == 0 {
		return notEmptyError("spec.reconfigure")
	}
	if reconfigure != nil {
		return r.validateReconfigureParams(ctx, k8sClient, cluster, reconfigure)
	}
	for _, reconfigure := range r.Spec.Reconfigures {
		if err := r.validateReconfigureParams(ctx, k8sClient, cluster, &reconfigure); err != nil {
			return err
		}
	}
	return nil
}

func (r *OpsRequest) validateReconfigureParams(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster,
	reconfigure *Reconfigure) error {
	if cluster.Spec.GetComponentByName(reconfigure.ComponentName) == nil {
		return fmt.Errorf("component %s not found", reconfigure.ComponentName)
	}
	for _, configuration := range reconfigure.Configurations {
		cmObj, err := r.getConfigMap(ctx, k8sClient, fmt.Sprintf("%s-%s-%s", r.Spec.GetClusterName(), reconfigure.ComponentName, configuration.Name))
		if err != nil {
			return err
		}
		for _, key := range configuration.Keys {
			// check add file
			if _, ok := cmObj.Data[key.Key]; !ok && key.FileContent == "" {
				return errors.Errorf("key %s not found in configmap %s", key.Key, configuration.Name)
			}
			if key.FileContent == "" && len(key.Parameters) == 0 {
				return errors.New("key.fileContent and key.parameters cannot be empty at the same time")
			}
		}
	}
	return nil
}

// compareRequestsAndLimits compares the resource requests and limits
func compareRequestsAndLimits(resources corev1.ResourceRequirements) (string, error) {
	requests := resources.Requests
	limits := resources.Limits
	if requests == nil || limits == nil {
		return "", nil
	}
	for k, v := range requests {
		if limitQuantity, ok := limits[k]; !ok {
			continue
		} else if compareQuantity(&v, &limitQuantity) {
			return v.String(), errors.New(fmt.Sprintf(`must be less than or equal to %s limit`, k))
		}
	}
	return "", nil
}

// compareQuantity compares requests quantity and limits quantity
func compareQuantity(requestQuantity, limitQuantity *resource.Quantity) bool {
	return requestQuantity != nil && limitQuantity != nil && requestQuantity.Cmp(*limitQuantity) > 0
}

// validateHorizontalScaling validates api when spec.type is HorizontalScaling
func (r *OpsRequest) validateHorizontalScaling(_ context.Context, _ client.Client, cluster *Cluster) error {
	horizontalScalingList := r.Spec.HorizontalScalingList
	if len(horizontalScalingList) == 0 {
		return notEmptyError("spec.horizontalScaling")
	}
	compOpsList := make([]ComponentOps, len(horizontalScalingList))
	hScaleMap := map[string]HorizontalScaling{}
	for i, v := range horizontalScalingList {
		compOpsList[i] = v.ComponentOps
		hScaleMap[v.ComponentName] = horizontalScalingList[i]
	}
	if err := r.checkComponentExistence(cluster, compOpsList); err != nil {
		return err
	}
	for _, comSpec := range cluster.Spec.ComponentSpecs {
		if hScale, ok := hScaleMap[comSpec.Name]; ok {
			if err := r.validateHorizontalScalingSpec(hScale, comSpec, cluster.Name, false); err != nil {
				return err
			}
		}
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if hScale, ok := hScaleMap[shardingSpec.Name]; ok {
			if err := r.validateHorizontalScalingSpec(hScale, shardingSpec.Template, cluster.Name, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// CountOfflineOrOnlineInstances calculate the number of instances that need to be brought online and offline corresponding to the instance template name.
func (r *OpsRequest) CountOfflineOrOnlineInstances(clusterName, componentName string, hScaleInstanceNames []string) map[string]int32 {
	offlineOrOnlineInsCountMap := map[string]int32{}
	for _, insName := range hScaleInstanceNames {
		insTplName := GetInstanceTemplateName(clusterName, componentName, insName)
		offlineOrOnlineInsCountMap[insTplName]++
	}
	return offlineOrOnlineInsCountMap
}

func (r *OpsRequest) validateHorizontalScalingSpec(hScale HorizontalScaling, compSpec ClusterComponentSpec, clusterName string, isSharding bool) error {
	scaleIn := hScale.ScaleIn
	scaleOut := hScale.ScaleOut
	if hScale.Replicas != nil && (scaleIn != nil || scaleOut != nil) {
		return fmt.Errorf(`"replicas" has been deprecated and cannot be used with "scaleOut" and "scaleIn"`)
	}
	if hScale.Replicas != nil {
		return nil
	}
	if lastCompConfiguration, ok := r.Status.LastConfiguration.Components[hScale.ComponentName]; ok {
		// use last component configuration snapshot
		compSpec.Instances = lastCompConfiguration.Instances
		compSpec.Replicas = *lastCompConfiguration.Replicas
		compSpec.OfflineInstances = lastCompConfiguration.OfflineInstances
	}
	compInsTplMap := map[string]int32{}
	for _, v := range compSpec.Instances {
		compInsTplMap[v.Name] = v.GetReplicas()
	}
	// Rules:
	// 1. length of offlineInstancesToOnline or onlineInstancesToOffline can't greater than the configured replicaChanges for the component.
	// 2. replicaChanges for component must greater than or equal to the sum of replicaChanges configured in instance templates.
	validateHScaleOperation := func(replicaChanger ReplicaChanger, newInstances []InstanceTemplate, offlineOrOnlineInsNames []string, isScaleIn bool) error {
		msgPrefix := "ScaleIn:"
		hScaleInstanceFieldName := "onlineInstancesToOffline"
		if !isScaleIn {
			msgPrefix = "ScaleOut:"
			hScaleInstanceFieldName = "offlineInstancesToOnline"
		}
		if isSharding && len(offlineOrOnlineInsNames) > 0 {
			return fmt.Errorf(`cannot specify %s for a sharding component "%s"`, hScaleInstanceFieldName, hScale.ComponentName)
		}
		if replicaChanger.ReplicaChanges != nil && len(offlineOrOnlineInsNames) > int(*replicaChanger.ReplicaChanges) {
			return fmt.Errorf(`the length of %s can't be greater than the "replicaChanges" for the component`, hScaleInstanceFieldName)
		}
		offlineOrOnlineInsCountMap := r.CountOfflineOrOnlineInstances(clusterName, hScale.ComponentName, offlineOrOnlineInsNames)
		insTplChangeMap := map[string]int32{}
		allReplicaChanges := int32(0)
		for _, v := range replicaChanger.Instances {
			compInsReplicas, ok := compInsTplMap[v.Name]
			if !ok {
				return fmt.Errorf(`%s cannot find the instance template "%s" in component "%s"`,
					msgPrefix, v.Name, hScale.ComponentName)
			}
			if isScaleIn && v.ReplicaChanges > compInsReplicas {
				return fmt.Errorf(`%s "replicaChanges" of instanceTemplate "%s" can't be greater than %d`,
					msgPrefix, v.Name, compInsReplicas)
			}
			allReplicaChanges += v.ReplicaChanges
			insTplChangeMap[v.Name] = v.ReplicaChanges
		}
		for insTplName, replicaCount := range offlineOrOnlineInsCountMap {
			replicaChanges, ok := insTplChangeMap[insTplName]
			if !ok {
				allReplicaChanges += replicaCount
				continue
			}
			if replicaChanges < replicaCount {
				return fmt.Errorf(`"replicaChanges" can't be less than %d when %d instances of the instance template "%s" are configured in %s`,
					replicaCount, replicaCount, insTplName, hScaleInstanceFieldName)
			}
		}
		for _, insTpl := range newInstances {
			if _, ok := compInsTplMap[insTpl.Name]; ok {
				return fmt.Errorf(`new instance template "%s" already exists in component "%s"`, insTpl.Name, hScale.ComponentName)
			}
			allReplicaChanges += insTpl.GetReplicas()
		}
		if replicaChanger.ReplicaChanges != nil && allReplicaChanges > *replicaChanger.ReplicaChanges {
			return fmt.Errorf(`%s "replicaChanges" can't be less than the sum of "replicaChanges" for specified instance templates`, msgPrefix)
		}
		return nil
	}
	if scaleIn != nil {
		if err := validateHScaleOperation(scaleIn.ReplicaChanger, nil, scaleIn.OnlineInstancesToOffline, true); err != nil {
			return err
		}
		if scaleIn.ReplicaChanges != nil && *scaleIn.ReplicaChanges > compSpec.Replicas {
			return fmt.Errorf(`"scaleIn.replicaChanges" can't be greater than %d for component "%s"`, compSpec.Replicas, hScale.ComponentName)
		}
	}
	if scaleOut != nil {
		if err := validateHScaleOperation(scaleOut.ReplicaChanger, scaleOut.NewInstances, scaleOut.OfflineInstancesToOnline, false); err != nil {
			return err
		}
		if len(scaleOut.OfflineInstancesToOnline) > 0 {
			offlineInstanceSet := sets.New(compSpec.OfflineInstances...)
			for _, offlineInsName := range scaleOut.OfflineInstancesToOnline {
				if _, ok := offlineInstanceSet[offlineInsName]; !ok {
					return fmt.Errorf(`cannot find the offline instance "%s" in component "%s" for scaleOut operation`, offlineInsName, hScale.ComponentName)
				}
			}
		}
	}
	return nil
}

// validateVolumeExpansion validates volumeExpansion api when spec.type is VolumeExpansion
func (r *OpsRequest) validateVolumeExpansion(ctx context.Context, cli client.Client, cluster *Cluster) error {
	volumeExpansionList := r.Spec.VolumeExpansionList
	if len(volumeExpansionList) == 0 {
		return notEmptyError("spec.volumeExpansion")
	}

	compOpsList := make([]ComponentOps, len(volumeExpansionList))
	for i, v := range volumeExpansionList {
		compOpsList[i] = v.ComponentOps
		var instanceNames []string
		for j := range v.Instances {
			instanceNames = append(instanceNames, v.Instances[j].Name)
		}
		if err := r.checkInstanceTemplate(cluster, v.ComponentOps, instanceNames); err != nil {
			return err
		}
	}
	if err := r.checkComponentExistence(cluster, compOpsList); err != nil {
		return err
	}
	return r.checkVolumesAllowExpansion(ctx, cli, cluster)
}

// validateSwitchover validates switchover api when spec.type is Switchover.
func (r *OpsRequest) validateSwitchover(ctx context.Context, cli client.Client, cluster *Cluster) error {
	switchoverList := r.Spec.SwitchoverList
	if len(switchoverList) == 0 {
		return notEmptyError("spec.switchover")
	}
	compOpsList := make([]ComponentOps, len(switchoverList))
	for i, v := range switchoverList {
		compOpsList[i] = v.ComponentOps

	}
	if err := r.checkComponentExistence(cluster, compOpsList); err != nil {
		return err
	}
	return validateSwitchoverResourceList(ctx, cli, cluster, switchoverList)
}

func (r *OpsRequest) checkInstanceTemplate(cluster *Cluster, componentOps ComponentOps, inputInstances []string) error {
	instanceNameMap := make(map[string]sets.Empty)
	setInstanceMap := func(instances []InstanceTemplate) {
		for i := range instances {
			instanceNameMap[instances[i].Name] = sets.Empty{}
		}
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if shardingSpec.Name != componentOps.ComponentName {
			continue
		}
		setInstanceMap(shardingSpec.Template.Instances)
	}
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name != componentOps.ComponentName {
			continue
		}
		setInstanceMap(compSpec.Instances)
	}
	var notFoundInstanceNames []string
	for _, insName := range inputInstances {
		if _, ok := instanceNameMap[insName]; !ok {
			notFoundInstanceNames = append(notFoundInstanceNames, insName)
		}
	}
	if len(notFoundInstanceNames) > 0 {
		return fmt.Errorf("instance: %v not found in cluster: %s", notFoundInstanceNames, r.Spec.GetClusterName())
	}
	return nil
}

// checkComponentExistence checks whether components to be operated exist in cluster spec.
func (r *OpsRequest) checkComponentExistence(cluster *Cluster, compOpsList []ComponentOps) error {
	compNameMap := make(map[string]sets.Empty)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compNameMap[compSpec.Name] = sets.Empty{}
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		compNameMap[shardingSpec.Name] = sets.Empty{}
	}
	var (
		notFoundCompNames []string
	)
	for _, compOps := range compOpsList {
		if _, ok := compNameMap[compOps.ComponentName]; !ok {
			notFoundCompNames = append(notFoundCompNames, compOps.ComponentName)
		}
		continue
	}

	if len(notFoundCompNames) > 0 {
		return fmt.Errorf("components: %v not found, in cluster.spec.componentSpecs or cluster.spec.shardingSpecs", notFoundCompNames)
	}
	return nil
}

func (r *OpsRequest) checkVolumesAllowExpansion(ctx context.Context, cli client.Client, cluster *Cluster) error {
	type Entity struct {
		existInSpec         bool
		storageClassName    *string
		allowExpansion      bool
		requestStorage      resource.Quantity
		isShardingComponent bool
	}

	vols := make(map[string]map[string]Entity)
	// component name/ sharding name -> vct name -> entity
	getKey := func(componentName string, templateName string) string {
		templateKey := ""
		if templateName != "" {
			templateKey = "." + templateName
		}
		return fmt.Sprintf("%s%s", componentName, templateKey)
	}
	setVols := func(vcts []OpsRequestVolumeClaimTemplate, componentName, templateName string) {
		for _, vct := range vcts {
			key := getKey(componentName, templateName)
			if _, ok := vols[key]; !ok {
				vols[key] = make(map[string]Entity)
			}
			vols[key][vct.Name] = Entity{false, nil, false, vct.Storage, false}
		}
	}

	for _, comp := range r.Spec.VolumeExpansionList {
		setVols(comp.VolumeClaimTemplates, comp.ComponentOps.ComponentName, "")
		for _, ins := range comp.Instances {
			setVols(ins.VolumeClaimTemplates, comp.ComponentOps.ComponentName, ins.Name)
		}
	}
	fillVol := func(vct ClusterComponentVolumeClaimTemplate, key string, isShardingComp bool) {
		e, ok := vols[key][vct.Name]
		if !ok {
			return
		}
		e.existInSpec = true
		e.storageClassName = vct.Spec.StorageClassName
		e.isShardingComponent = isShardingComp
		vols[key][vct.Name] = e
	}
	fillCompVols := func(compSpec ClusterComponentSpec, componentName string, isShardingComp bool) {
		key := getKey(componentName, "")
		if _, ok := vols[key]; !ok {
			return // ignore not-exist component
		}
		for _, vct := range compSpec.VolumeClaimTemplates {
			fillVol(vct, key, isShardingComp)
		}
		for _, ins := range compSpec.Instances {
			key = getKey(componentName, ins.Name)
			for _, vct := range ins.VolumeClaimTemplates {
				fillVol(vct, key, isShardingComp)
			}
		}
	}
	// traverse the spec to update volumes
	for _, comp := range cluster.Spec.ComponentSpecs {
		fillCompVols(comp, comp.Name, false)
	}
	for _, sharding := range cluster.Spec.ShardingSpecs {
		fillCompVols(sharding.Template, sharding.Name, true)
	}

	// check all used storage classes
	var err error
	for key, compVols := range vols {
		for vname := range compVols {
			e := vols[key][vname]
			if !e.existInSpec {
				continue
			}
			e.storageClassName, err = r.getSCNameByPvcAndCheckStorageSize(ctx, cli, key, vname, e.isShardingComponent, e.requestStorage)
			if err != nil {
				return err
			}
			allowExpansion, err := r.checkStorageClassAllowExpansion(ctx, cli, e.storageClassName)
			if err != nil {
				continue // ignore the error and take it as not-supported
			}
			e.allowExpansion = allowExpansion
			vols[key][vname] = e
		}
	}

	for key, compVols := range vols {
		var (
			notFound     []string
			notSupport   []string
			notSupportSc []string
		)
		for vct, e := range compVols {
			if !e.existInSpec {
				notFound = append(notFound, vct)
			}
			if !e.allowExpansion {
				notSupport = append(notSupport, vct)
				if e.storageClassName != nil {
					notSupportSc = append(notSupportSc, *e.storageClassName)
				}
			}
		}
		if len(notFound) > 0 {
			return fmt.Errorf("volumeClaimTemplates: %v not found in component: %s, you can view infos by command: "+
				"kubectl get cluster %s -n %s", notFound, key, cluster.Name, r.Namespace)
		}
		if len(notSupport) > 0 {
			var notSupportScString string
			if len(notSupportSc) > 0 {
				notSupportScString = fmt.Sprintf("storageClass: %v of ", notSupportSc)
			}
			return fmt.Errorf(notSupportScString+"volumeClaimTemplate: %v not support volume expansion in component: %s, you can view infos by command: "+
				"kubectl get sc", notSupport, key)
		}
	}
	return nil
}

// checkStorageClassAllowExpansion checks whether the specified storage class supports volume expansion.
func (r *OpsRequest) checkStorageClassAllowExpansion(ctx context.Context,
	cli client.Client,
	storageClassName *string) (bool, error) {
	if storageClassName == nil {
		return false, nil
	}
	storageClass := &storagev1.StorageClass{}
	// take not found error as unsupported
	if err := cli.Get(ctx, types.NamespacedName{Name: *storageClassName}, storageClass); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if storageClass.AllowVolumeExpansion == nil {
		return false, nil
	}
	return *storageClass.AllowVolumeExpansion, nil
}

// getSCNameByPvcAndCheckStorageSize gets the storageClassName by pvc and checks if the storage size is valid.
func (r *OpsRequest) getSCNameByPvcAndCheckStorageSize(ctx context.Context,
	cli client.Client,
	componentName,
	vctName string,
	isShardingComponent bool,
	requestStorage resource.Quantity) (*string, error) {
	matchingLabels := client.MatchingLabels{
		constant.AppInstanceLabelKey:             r.Spec.GetClusterName(),
		constant.VolumeClaimTemplateNameLabelKey: vctName,
	}
	if isShardingComponent {
		matchingLabels[constant.KBAppShardingNameLabelKey] = componentName
	} else {
		matchingLabels[constant.KBAppComponentLabelKey] = componentName
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(ctx, pvcList, client.InNamespace(r.Namespace), matchingLabels); err != nil {
		return nil, err
	}
	if len(pvcList.Items) == 0 {
		return nil, nil
	}
	pvc := pvcList.Items[0]
	previousValue := *pvc.Status.Capacity.Storage()
	if requestStorage.Cmp(previousValue) < 0 {
		return nil, fmt.Errorf(`requested storage size of volumeClaimTemplate "%s" can not less than status.capacity.storage "%s" `,
			vctName, previousValue.String())
	}
	return pvc.Spec.StorageClassName, nil
}

// validateDataScript validates the data script.
func (r *OpsRequest) validateDataScript(ctx context.Context, cli client.Client, cluster *Cluster) error {
	validateScript := func(spec *ScriptSpec) error {
		rawScripts := spec.Script
		scriptsFrom := spec.ScriptFrom
		if len(rawScripts) == 0 && (scriptsFrom == nil) {
			return fmt.Errorf("spec.scriptSpec.script and spec.scriptSpec.scriptFrom can not be empty at the same time")
		}
		if scriptsFrom != nil {
			if scriptsFrom.ConfigMapRef == nil && scriptsFrom.SecretRef == nil {
				return fmt.Errorf("spec.scriptSpec.scriptFrom.configMapRefs and spec.scriptSpec.scriptFrom.secretRefs can not be empty at the same time")
			}
			for _, configMapRef := range scriptsFrom.ConfigMapRef {
				if err := cli.Get(ctx, types.NamespacedName{Name: configMapRef.Name, Namespace: r.Namespace}, &corev1.ConfigMap{}); err != nil {
					return err
				}
			}
			for _, secret := range scriptsFrom.SecretRef {
				if err := cli.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: r.Namespace}, &corev1.Secret{}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	scriptSpec := r.Spec.ScriptSpec
	if scriptSpec == nil {
		return notEmptyError("spec.scriptSpec")
	}

	if err := r.checkComponentExistence(cluster, []ComponentOps{scriptSpec.ComponentOps}); err != nil {
		return err
	}

	if err := validateScript(scriptSpec); err != nil {
		return err
	}

	return nil
}

// validateVerticalResourceList checks if k8s resourceList is legal
func validateVerticalResourceList(resourceList map[corev1.ResourceName]resource.Quantity) (string, error) {
	for k := range resourceList {
		if k != corev1.ResourceCPU && k != corev1.ResourceMemory && !strings.HasPrefix(k.String(), corev1.ResourceHugePagesPrefix) {
			return string(k), fmt.Errorf("resource key is not cpu or memory or hugepages- ")
		}
	}

	return "", nil
}

func notEmptyError(target string) error {
	return fmt.Errorf(`"%s" can not be empty`, target)
}

func invalidValueError(target string, value string) error {
	return fmt.Errorf(`invalid value for "%s": %s`, target, value)
}

// GetRunningOpsByOpsType gets the running opsRequests by type.
func GetRunningOpsByOpsType(ctx context.Context, cli client.Client,
	clusterName, namespace, opsType string) ([]OpsRequest, error) {
	opsRequestList := &OpsRequestList{}
	if err := cli.List(ctx, opsRequestList, client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.OpsRequestTypeLabelKey: opsType,
	}, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	if len(opsRequestList.Items) == 0 {
		return nil, nil
	}
	var runningOpsList []OpsRequest
	for _, v := range opsRequestList.Items {
		if v.Status.Phase == OpsRunningPhase {
			runningOpsList = append(runningOpsList, v)
			break
		}
	}
	return runningOpsList, nil
}

// validateSwitchoverResourceList checks if switchover resourceList is legal.
func validateSwitchoverResourceList(ctx context.Context, cli client.Client, cluster *Cluster, switchoverList []Switchover) error {
	var (
		targetRole string
	)
	for _, switchover := range switchoverList {
		if switchover.InstanceName == "" {
			return notEmptyError("switchover.instanceName")
		}

		// TODO(xingran): this will be removed in the future.
		validateBaseOnClusterCompDef := func(clusterCmpDef string) error {
			// check clusterComponentDefinition whether support switchover
			clusterCompDefObj, err := getClusterComponentDefByName(ctx, cli, *cluster, clusterCmpDef)
			if err != nil {
				return err
			}
			if clusterCompDefObj == nil {
				return fmt.Errorf("this cluster component %s is invalid", switchover.ComponentName)
			}
			if clusterCompDefObj.SwitchoverSpec == nil {
				return fmt.Errorf("this cluster component %s does not support switchover", switchover.ComponentName)
			}
			switch switchover.InstanceName {
			case KBSwitchoverCandidateInstanceForAnyPod:
				if clusterCompDefObj.SwitchoverSpec.WithoutCandidate == nil {
					return fmt.Errorf("this cluster component %s does not support promote without specifying an instance. Please specify a specific instance for the promotion", switchover.ComponentName)
				}
			default:
				if clusterCompDefObj.SwitchoverSpec.WithCandidate == nil {
					return fmt.Errorf("this cluster component %s does not support specifying an instance for promote. If you want to perform a promote operation, please do not specify an instance", switchover.ComponentName)
				}
			}
			// check switchover.InstanceName whether exist and role label is correct
			if switchover.InstanceName == KBSwitchoverCandidateInstanceForAnyPod {
				return nil
			}
			pod := &corev1.Pod{}
			if err := cli.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: switchover.InstanceName}, pod); err != nil {
				return fmt.Errorf("get instanceName %s failed, err: %s, and check the validity of the instanceName using \"kbcli cluster list-instances\"", switchover.InstanceName, err.Error())
			}
			v, ok := pod.Labels[constant.RoleLabelKey]
			if !ok || v == "" {
				return fmt.Errorf("instanceName %s cannot be promoted because it had a invalid role label", switchover.InstanceName)
			}
			if v == constant.Primary || v == constant.Leader {
				return fmt.Errorf("instanceName %s cannot be promoted because it is already the primary or leader instance", switchover.InstanceName)
			}
			if !strings.HasPrefix(pod.Name, fmt.Sprintf("%s-%s", cluster.Name, switchover.ComponentName)) {
				return fmt.Errorf("instanceName %s does not belong to the current component, please check the validity of the instance using \"kbcli cluster list-instances\"", switchover.InstanceName)
			}
			return nil
		}

		validateBaseOnCompDef := func(compDef string) error {
			getTargetRole := func(roles []ReplicaRole) (string, error) {
				targetRole = ""
				if len(roles) == 0 {
					return targetRole, errors.New("component has no roles definition, does not support switchover")
				}
				for _, role := range roles {
					if role.Serviceable && role.Writable {
						if targetRole != "" {
							return targetRole, errors.New("componentDefinition has more than role is serviceable and writable, does not support switchover")
						}
						targetRole = role.Name
					}
				}
				return targetRole, nil
			}
			compDefObj, err := getComponentDefByName(ctx, cli, compDef)
			if err != nil {
				return err
			}
			if compDefObj == nil {
				return fmt.Errorf("this component %s referenced componentDefinition is invalid", switchover.ComponentName)
			}
			if compDefObj.Spec.LifecycleActions == nil || compDefObj.Spec.LifecycleActions.Switchover == nil {
				return fmt.Errorf("this cluster component %s does not support switchover", switchover.ComponentName)
			}
			switch switchover.InstanceName {
			case KBSwitchoverCandidateInstanceForAnyPod:
				if compDefObj.Spec.LifecycleActions.Switchover.WithoutCandidate == nil {
					return fmt.Errorf("this cluster component %s does not support promote without specifying an instance. Please specify a specific instance for the promotion", switchover.ComponentName)
				}
			default:
				if compDefObj.Spec.LifecycleActions.Switchover.WithCandidate == nil {
					return fmt.Errorf("this cluster component %s does not support specifying an instance for promote. If you want to perform a promote operation, please do not specify an instance", switchover.ComponentName)
				}
			}
			// check switchover.InstanceName whether exist and role label is correct
			if switchover.InstanceName == KBSwitchoverCandidateInstanceForAnyPod {
				return nil
			}
			targetRole, err = getTargetRole(compDefObj.Spec.Roles)
			if err != nil {
				return err
			}
			if targetRole == "" {
				return errors.New("componentDefinition has no role is serviceable and writable, does not support switchover")
			}
			pod := &corev1.Pod{}
			if err := cli.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: switchover.InstanceName}, pod); err != nil {
				return fmt.Errorf("get instanceName %s failed, err: %s, and check the validity of the instanceName using \"kbcli cluster list-instances\"", switchover.InstanceName, err.Error())
			}
			v, ok := pod.Labels[constant.RoleLabelKey]
			if !ok || v == "" {
				return fmt.Errorf("instanceName %s cannot be promoted because it had a invalid role label", switchover.InstanceName)
			}
			if v == targetRole {
				return fmt.Errorf("instanceName %s cannot be promoted because it is already the primary or leader instance", switchover.InstanceName)
			}
			if !strings.HasPrefix(pod.Name, fmt.Sprintf("%s-%s", cluster.Name, switchover.ComponentName)) {
				return fmt.Errorf("instanceName %s does not belong to the current component, please check the validity of the instance using \"kbcli cluster list-instances\"", switchover.InstanceName)
			}
			return nil
		}

		compSpec := cluster.Spec.GetComponentByName(switchover.ComponentName)
		if compSpec == nil {
			return fmt.Errorf("component %s not found", switchover.ComponentName)
		}
		if compSpec.ComponentDef != "" {
			return validateBaseOnCompDef(compSpec.ComponentDef)
		} else {
			return validateBaseOnClusterCompDef(cluster.Spec.GetComponentDefRefName(switchover.ComponentName))
		}
	}
	return nil
}

// getComponentDefByName gets ComponentDefinition with compDefName
func getComponentDefByName(ctx context.Context, cli client.Client, compDefName string) (*ComponentDefinition, error) {
	compDef := &ComponentDefinition{}
	if err := cli.Get(ctx, types.NamespacedName{Name: compDefName}, compDef); err != nil {
		return nil, err
	}
	return compDef, nil
}

// getClusterComponentDefByName gets component from ClusterDefinition with compDefName
func getClusterComponentDefByName(ctx context.Context, cli client.Client, cluster Cluster,
	compDefName string) (*ClusterComponentDefinition, error) {
	clusterDef := &ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return nil, err
	}
	for _, component := range clusterDef.Spec.ComponentDefs {
		if component.Name == compDefName {
			return &component, nil
		}
	}
	return nil, ErrNotMatchingCompDef
}
