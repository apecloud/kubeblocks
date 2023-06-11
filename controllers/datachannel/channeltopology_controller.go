package datachannel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	datachannelv1alpha1 "github.com/apecloud/kubeblocks/apis/datachannel/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ChannelTopologyReconciler reconciles a ChannelTopology object
type ChannelTopologyReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *ChannelTopologyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("channelTopology", req.NamespacedName),
		Recorder: r.Recorder,
	}

	// Todo: add finalizers, modify label and annotation when terminating
	// Todo: how to support modified the channel
	return r.handler(reqCtx,
		r.listDependResource,
		r.waitDependResourceReady,
		r.validateTopology,
		r.clusterConfigSettings,
		r.generateChannelWorkerBuildingPlan,
		r.executeChannelWorkerBuildingPlan,
	)
}

func (r *ChannelTopologyReconciler) handler(reqCtx intctrlutil.RequestCtx, steps ...func(c *ChannelTopologyHandler) (*ctrl.Result, error)) (ctrl.Result, error) {
	channelTopologyHandler := r.initializationHandler(reqCtx)

	var result *ctrl.Result
	var err error
	for _, step := range steps {
		result, err = step(channelTopologyHandler)
		if result != nil {
			return *result, err
		}
	}

	return intctrlutil.Reconciled()
}

func (r *ChannelTopologyReconciler) initializationHandler(Request intctrlutil.RequestCtx) *ChannelTopologyHandler {
	return &ChannelTopologyHandler{
		&ChannelTopologyResources{},
		Request,
		ctrl.Result{},
		nil,
	}
}

func (r *ChannelTopologyReconciler) listDependResource(c *ChannelTopologyHandler) (*ctrl.Result, error) {
	logClusterIfNeeded := func(err error, namespacedName types.NamespacedName) {
		if errors.IsNotFound(err) {
			c.Request.Log.Info(fmt.Sprintf("Cluster:[%s-%s] is not found.",
				namespacedName.Namespace,
				namespacedName.Name))
		} else if err != nil {
			c.Request.Log.Error(err, err.Error())
		}
	}

	var err error = nil

	// list by client-go
	c.Resources.ChannelTopology = &datachannelv1alpha1.ChannelTopology{}

	err = r.Client.Get(c.Request.Ctx, c.Request.Req.NamespacedName, c.Resources.ChannelTopology)
	if err != nil && c.Resources.ChannelTopology.Status.Phase != datachannelv1alpha1.FailedChannelTopologyPhase {
		msg := fmt.Sprintf("channel topology:[%s] is not exist.", c.Request.Req.Name)
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, c.Request.Log, msg))
	} else if c.Resources.ChannelTopology.Status.Phase == datachannelv1alpha1.FailedChannelTopologyPhase {
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}

	// build default setting
	buildChannelTopologyWithDefaultSettings(c.Resources.ChannelTopology)

	// build with channels
	if len(c.Resources.ChannelTopology.Spec.Channels) == 0 {
		c.Request.Log.Info(fmt.Sprintf("channelTopology:[%s-%s] has no channel inside.",
			c.Resources.ChannelTopology.Namespace,
			c.Resources.ChannelTopology.Name))
		return nil, nil
	}
	c.Resources.ChannelResources = make(map[string]ChannelResources)
	for _, channel := range c.Resources.ChannelTopology.Spec.Channels {
		channelResources := ChannelResources{
			ChannelName:         channel.Name,
			ChannelTopologyName: c.Resources.ChannelTopology.Name,
			ChannelSyncObjs:     channel.IncludeObjs,

			SourceChannelDefName: channel.From.ClusterRef,
			SinkChannelDefName:   channel.To.ChannelDefinitionRef,
		}

		namespacedName := types.NamespacedName{
			Namespace: c.Request.Req.Namespace,
		}
		channelResources.SourceNodeCluster = &appv1alpha1.Cluster{}
		namespacedName.Name = channel.From.ClusterRef
		err = r.Client.Get(c.Request.Ctx, namespacedName, channelResources.SourceNodeCluster)
		logClusterIfNeeded(err, namespacedName)

		channelResources.SinkNodeCluster = &appv1alpha1.Cluster{}
		namespacedName.Name = channel.To.ClusterRef
		err = r.Client.Get(c.Request.Ctx, namespacedName, channelResources.SinkNodeCluster)
		logClusterIfNeeded(err, namespacedName)

		// Todo: support hub

		c.Resources.ChannelResources[channel.Name] = channelResources
	}

	return nil, nil
}

func (r *ChannelTopologyReconciler) waitDependResourceReady(c *ChannelTopologyHandler) (*ctrl.Result, error) {
	checkIfWaitingTimeout := func() bool {
		if c.Resources.ChannelTopology.Status.Phase != datachannelv1alpha1.PreparingChannelTopologyPhase {
			return false
		}
		ttlMinutes := time.Duration(c.Resources.ChannelTopology.Spec.Settings.Topology.PrepareTTLMinutes)
		if time.Now().Sub(c.Resources.ChannelTopology.CreationTimestamp.Time) > ttlMinutes*time.Minute {
			return true
		}
		return false
	}

	checkWhenDependClustersNotReady := func(msg string) (*ctrl.Result, error) {
		var result *ctrl.Result = nil
		var err error

		if checkIfWaitingTimeout() {
			if err = r.topologyStatusBuilding(c.Resources.ChannelTopology, c.Request, datachannelv1alpha1.FailedChannelTopologyPhase, msg); err != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			}
			result = &ctrl.Result{}
		} else if c.Resources.ChannelTopology.Status.Phase != datachannelv1alpha1.PreparingChannelTopologyPhase {
			if err = r.topologyStatusBuilding(c.Resources.ChannelTopology, c.Request, datachannelv1alpha1.PreparingChannelTopologyPhase, msg); err != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			}
			result = &ctrl.Result{}
		} else {
			result, _ = intctrlutil.ResultToP(intctrlutil.RequeueAfter(5*time.Second, c.Request.Log, msg))
		}

		return result, nil
	}

	// cluster not exist
	var msg string
	if nonExistedClusters := c.findEmptyResources(); len(nonExistedClusters) > 0 {
		msg = fmt.Sprintf("cluster:[%s] not existed yet.", strings.Join(nonExistedClusters, ","))
		if result, err := checkWhenDependClustersNotReady(msg); result != nil {
			return result, err
		}
	}

	// check resource status
	if notReadyClusters := c.findStatusUnMatchResources(appv1alpha1.RunningClusterPhase); len(notReadyClusters) > 0 {
		msg = fmt.Sprintf("cluster:[%s] not ready yet.", strings.Join(notReadyClusters, ","))
		if result, err := checkWhenDependClustersNotReady(msg); result != nil {
			return result, err
		}
	}

	return nil, nil
}

func (r *ChannelTopologyReconciler) validateTopology(c *ChannelTopologyHandler) (*ctrl.Result, error) {
	// get cluster depend-on channelDefinition
	var err error
	var result *ctrl.Result

	// find ChannelDefinition with ChannelTopology.Spec setting or with Default ChannelDefinition setting
	buildChannelDef := func(channelDefName string, clusterName string) (*ctrl.Result, error, *datachannelv1alpha1.ChannelDefinition) {
		channelDef := datachannelv1alpha1.ChannelDefinition{}

		channelDef, err = r.findChannelDef(c.Request, channelDefName, clusterName)
		if err != nil {
			result, err = intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			return result, err, &channelDef
		}
		if isClientObjectEmpty(&channelDef) {
			if err = r.topologyStatusBuilding(c.Resources.ChannelTopology, c.Request,
				datachannelv1alpha1.FailedChannelTopologyPhase,
				fmt.Sprintf("Cluster:[%s] has no channel-definition:[%s] in Topology:[%s].",
					clusterName, channelDefName, c.Resources.ChannelTopology.Name)); err != nil {
				result, err = intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			} else {
				result, err = intctrlutil.ResultToP(intctrlutil.Reconciled())
			}
			return result, err, &channelDef
		}
		return nil, nil, &channelDef
	}

	// validate topology struct limit and build police
	isChannelDefTopologyStructsValid := func(structs []datachannelv1alpha1.TopologyStruct, targetStruct datachannelv1alpha1.TopologyStruct) bool {
		if len(structs) == 0 {
			// empty setting means support every topology structs.
			return true
		}
		for _, s := range structs {
			if s == targetStruct {
				return true
			}
		}
		return false
	}

	for channelName, channelObj := range c.Resources.ChannelResources {
		result, err, channelObj.SourceChannelDefinition = buildChannelDef(channelObj.SourceChannelDefName, channelObj.SourceNodeCluster.Spec.ClusterDefRef)
		if err != nil || result != nil {
			return result, err
		}

		result, err, channelObj.SinkChannelDefinition = buildChannelDef(channelObj.SinkChannelDefName, channelObj.SinkNodeCluster.Spec.ClusterDefRef)
		if err != nil || result != nil {
			return result, err
		}

		// Todo: support hub

		c.Resources.ChannelResources[channelName] = channelObj
	}

	errMsgs := make([]string, 0)
	for _, channel := range c.Resources.ChannelResources {
		if !isChannelDefTopologyStructsValid(channel.SourceChannelDefinition.Spec.TopologyStructs,
			c.Resources.ChannelTopology.Spec.Settings.Topology.TopologyStruct) {
			errMsgs = append(errMsgs, fmt.Sprintf("cluster:[%s] channel definition is not support Topology-struct:[%s].",
				channel.SourceNodeCluster.Name,
				c.Resources.ChannelTopology.Spec.Settings.Topology.TopologyStruct))
		}
	}

	isValidate, msg := validateTopologyStruct(c.Resources.ChannelTopology.Spec.Channels,
		c.Resources.ChannelTopology.Spec.Settings.Topology.TopologyStruct)
	if !isValidate {
		if msg == "" {
			msg = fmt.Sprintf("channel topology meet circle when topology-struct is [%s].",
				c.Resources.ChannelTopology.Spec.Settings.Topology.TopologyStruct)
		}
		errMsgs = append(errMsgs, msg)
	}

	if len(errMsgs) > 0 {
		resultMsg := strings.Join(errMsgs, ";")
		if err = r.topologyStatusBuilding(c.Resources.ChannelTopology, c.Request, datachannelv1alpha1.FailedChannelTopologyPhase, resultMsg); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}

	return nil, nil
}

func (r *ChannelTopologyReconciler) clusterConfigSettings(c *ChannelTopologyHandler) (*ctrl.Result, error) {
	var err error
	// find the channel which is not created
	channelClusters := &appv1alpha1.ClusterList{}
	if err = r.Client.List(c.Request.Ctx, channelClusters,
		client.InNamespace(c.Request.Req.Namespace),
		client.MatchingLabels(buildChannelTopologyLabels(c.Resources.ChannelTopology))); err != nil {
		if statusUptErr := r.topologyStatusBuilding(c.Resources.ChannelTopology, c.Request,
			datachannelv1alpha1.FailedChannelTopologyPhase,
			"no cluster with Channel-topology label."); statusUptErr != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(statusUptErr, c.Request.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
	}

	// create the OpsRequest
	notCreatedChannelName := make([]string, 0)
specChannelLoop:
	for _, channel := range c.Resources.ChannelTopology.Spec.Channels {
		for _, cluster := range channelClusters.Items {
			// sink cluster channel is created means the channel is created.
			if isChannelClusterAnnotationsMatch(cluster.Annotations, channel.Name, false) {
				continue specChannelLoop
			}
		}
		notCreatedChannelName = append(notCreatedChannelName, channel.Name)
	}

	// check the source cluster finished reconfiguring ops or not
	notCreatedSourceClusterChannelMap := make(map[string]ChannelResources)
	notReadySourceClusterChannelMap := make(map[string]ChannelResources)

notCreatedChannelLoop:
	for _, channelName := range notCreatedChannelName {
		channelResources := c.Resources.ChannelResources[channelName]

		opsList := &appv1alpha1.OpsRequestList{}
		if err = r.Client.List(c.Request.Ctx, opsList,
			client.InNamespace(c.Request.Req.Namespace),
			client.MatchingLabels(buildClusterNameLabels(channelResources.SourceNodeCluster.Name))); err != nil {
			if errors.IsNotFound(err) {
				notCreatedSourceClusterChannelMap[channelResources.SourceNodeCluster.Name] = channelResources
				continue
			} else {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			}
		}
		hasOps := false
		for _, ops := range opsList.Items {
			// Todo: retry when reconfiguring ops failed
			if ops.Spec.Type == appv1alpha1.ReconfiguringType &&
				isOpsAnnotationsMatch(ops.Annotations, channelName) {
				hasOps = true
				if ops.Status.Phase == appv1alpha1.OpsSucceedPhase {
					continue notCreatedChannelLoop
				}
			}
		}
		if hasOps {
			notReadySourceClusterChannelMap[channelResources.SourceNodeCluster.Name] = channelResources
		} else {
			notCreatedSourceClusterChannelMap[channelResources.SourceNodeCluster.Name] = channelResources
		}
	}

	// update status if needed
	if c.Resources.ChannelTopology.Status.ChannelTotal != len(c.Resources.ChannelTopology.Spec.Channels) ||
		c.Resources.ChannelTopology.Status.ChannelWaitForBuilding != len(notCreatedChannelName) {
		newStatus := datachannelv1alpha1.ChannelTopologyStatus{
			Phase:                  datachannelv1alpha1.RunningChannelTopologyPhase,
			ChannelTotal:           len(c.Resources.ChannelTopology.Spec.Channels),
			ChannelEstablished:     len(c.Resources.ChannelTopology.Spec.Channels) - len(notCreatedChannelName),
			ChannelWaitForBuilding: len(notCreatedChannelName),
		}
		if len(notCreatedChannelName) > 0 {
			newStatus.Phase = datachannelv1alpha1.PreparingChannelTopologyPhase
		}
		if err = r.topologyStatusBuildingWithSpecify(c.Resources.ChannelTopology, c.Request, newStatus); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
		} else {
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}
	}

	// only when reconfiguring ops succeed will trigger next actions
	if len(notCreatedSourceClusterChannelMap) > 0 || len(notReadySourceClusterChannelMap) > 0 {
		// create reconfiguring ops for notCreatedSourceClusterChannelMap
		for _, channelResource := range notCreatedSourceClusterChannelMap {
			if err = r.Client.Create(c.Request.Ctx, buildOpsRequestForChannel(c.Resources.ChannelTopology, &channelResource)); err != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			}
		}
		// wait for ops finish
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}

	c.Request.Log.Info(fmt.Sprintf("channels in topology:[%s] source cluster 's reconfiguring ops finished.", c.Resources.ChannelTopology.Name))
	return nil, nil
}

// Todo: compare with current channel cluster and topology settings, remove channel
func (r *ChannelTopologyReconciler) generateChannelWorkerBuildingPlan(c *ChannelTopologyHandler) (*ctrl.Result, error) {
	topologyWorkerClusterList := &appv1alpha1.ClusterList{}
	if err := r.Client.List(c.Request.Ctx, topologyWorkerClusterList,
		client.InNamespace(c.Request.Req.Namespace),
		client.MatchingLabels(buildChannelTopologyLabels(c.Resources.ChannelTopology))); err != nil && !errors.IsNotFound(err) {
		return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
	}

	filterClusterWithCondition := func(channelName string, clusterName string, isSource bool) *appv1alpha1.Cluster {
		for _, cluster := range topologyWorkerClusterList.Items {
			if isChannelClusterAnnotationsMatch(cluster.Annotations, channelName, isSource) {
				// The annotation matches and the specified cluster name matches
				return &cluster
			}
		}
		return &appv1alpha1.Cluster{}
	}

	for _, channel := range c.Resources.ChannelResources {
		// generate source channel plan
		sourceChannelImpl := ChannelResources{}
		if isClientObjectEmpty(filterClusterWithCondition(channel.ChannelName, channel.SourceNodeCluster.Name, true)) {
			needCreateSource := true
			if c.Resources.ChannelTopology.Spec.Settings.Topology.BuildingPolicy == datachannelv1alpha1.ClusterPriorityBuildingPolicy {
				// If the cluster-priority strategy is selected, the channel on the source side will be reused as much as possible
				// Todo: reuse source cluster
				sourceImpl := &appv1alpha1.Cluster{}
				if !isClientObjectEmpty(sourceImpl) {
					implSourceChannelName := sourceImpl.Annotations[constant.ChannelNameAnnotationKey]
					sourceChannelImpl = c.Resources.ChannelResources[implSourceChannelName]
				}
			}
			if needCreateSource {
				c.Resources.ChannelBuildingPlans = append(c.Resources.ChannelBuildingPlans, ChannelBuildingPlan{
					ChannelResources: channel,
					isBuildingSource: true,
				})
			}
		}

		// generate sink channel plan
		sinkChannelCluster := filterClusterWithCondition(channel.ChannelName, channel.SinkNodeCluster.Name, false)
		if isClientObjectEmpty(sinkChannelCluster) {
			c.Resources.ChannelBuildingPlans = append(c.Resources.ChannelBuildingPlans, ChannelBuildingPlan{
				ChannelImpl:      sourceChannelImpl,
				ChannelResources: channel,
				isBuildingSource: false,
			})
		}

	}
	return nil, nil
}

func (r *ChannelTopologyReconciler) executeChannelWorkerBuildingPlan(c *ChannelTopologyHandler) (*ctrl.Result, error) {
	isClusterMarkedBuilding := func(cluster *appv1alpha1.Cluster, topologyName string, channelName string, isSource bool) (bool, string) {
		errMsg := ""
		if labelValue := cluster.Labels[constant.ChannelTopologyLabelKey]; labelValue != "" && labelValue != topologyName {
			errMsg = fmt.Sprintf("cluster:[%s] is used for channel topology:[%s], can't use for another topology.",
				cluster.Name, labelValue)
			return false, errMsg
		} else if labelValue == "" {
			return false, ""
		}
		return isClusterAnnotationsMatch(cluster.Annotations, channelName, isSource), ""
	}

	markClusterWithChannel := func(cluster *appv1alpha1.Cluster, topologyName string, channelName string, isSource bool) error {
		clusterObj := client.MergeFrom(cluster.DeepCopy())

		if cluster.Labels == nil {
			cluster.Labels = make(map[string]string)
		}
		cluster.Labels[constant.ChannelTopologyLabelKey] = topologyName

		channelAnnotationKey := constant.ChannelSourceRelationAnnotationKey
		if !isSource {
			channelAnnotationKey = constant.ChannelSinkRelationAnnotationKey
		}
		channelNames := make([]string, 0)
		if _, ok := cluster.Annotations[channelAnnotationKey]; ok {
			channelNames = append(channelNames, strings.Split(cluster.Annotations[channelAnnotationKey], ",")...)
		}
		isContained := false
		for _, name := range channelNames {
			if name == channelName {
				isContained = true
				break
			}
		}
		if !isContained {
			channelNames = append(channelNames, channelName)
			if cluster.Annotations == nil {
				cluster.Annotations = make(map[string]string)
			}
			cluster.Annotations[channelAnnotationKey] = strings.Join(channelNames, ",")
		}

		return r.Client.Patch(c.Request.Ctx, cluster, clusterObj)
	}

	// patch source with annotation, witch channel's rely cluster has target annotation, will do building
	var cluster *appv1alpha1.Cluster
	var err error
	for _, plan := range c.Resources.ChannelBuildingPlans {
		// topology label, and annotation: role & channel(ChannelSourceRelationAnnotationKey, ChannelSinkRelationAnnotationKey)
		if plan.isBuildingSource {
			cluster = plan.ChannelResources.SourceNodeCluster
		} else {
			cluster = plan.ChannelResources.SinkNodeCluster
		}

		isMarked, errMsg := isClusterMarkedBuilding(cluster, c.Resources.ChannelTopology.Name,
			plan.ChannelResources.ChannelName,
			plan.isBuildingSource)
		if errMsg != "" {
			if err = r.topologyStatusBuilding(c.Resources.ChannelTopology,
				c.Request,
				datachannelv1alpha1.FailedChannelTopologyPhase,
				errMsg); err != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			}
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}

		if !isMarked {
			if err = markClusterWithChannel(cluster,
				c.Resources.ChannelTopology.Name,
				plan.ChannelResources.ChannelName,
				plan.isBuildingSource); err != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
			}
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}

		// building channel one by one with status change event's reconcile
		err = plan.buildingInit(c.Resources.ChannelTopology).
			buildOwnerReference(c.Resources.ChannelTopology, c.Request).
			buildingWithAccount(r.Client, c.Request).
			buildingWithRelyClusters().
			buildingWithSyncObjs().
			buildingWithExtraEnvs().
			transform()

		if err != nil {
			if statusUptErr := r.topologyStatusBuilding(c.Resources.ChannelTopology,
				c.Request,
				datachannelv1alpha1.FailedChannelTopologyPhase,
				err.Error()); statusUptErr != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(statusUptErr, c.Request.Log, ""))
			}
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
		}

		// do create with preBuildCluster  Todo: create完还为触发一次导致isExisted报错
		if err = r.Client.Create(c.Request.Ctx, &plan.BuildingCluster); err != nil {
			if statusUptErr := r.topologyStatusBuilding(c.Resources.ChannelTopology,
				c.Request,
				datachannelv1alpha1.FailedChannelTopologyPhase,
				err.Error()); statusUptErr != nil {
				return intctrlutil.ResultToP(intctrlutil.RequeueWithError(statusUptErr, c.Request.Log, ""))
			}
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, c.Request.Log, ""))
		} else {
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}
		// Todo: channel count status
	}

	return nil, nil
}

func (r *ChannelTopologyReconciler) topologyStatusBuilding(topology *datachannelv1alpha1.ChannelTopology,
	request intctrlutil.RequestCtx,
	phase datachannelv1alpha1.ChannelTopologyPhase,
	msg string) error {

	statusPatch := client.MergeFrom(topology.DeepCopy())
	topology.Status.Phase = phase
	topology.Status.Message = msg
	topology.Status.ChannelTotal = len(topology.Spec.Channels)

	return r.Client.Status().Patch(request.Ctx, topology, statusPatch)
}

func (r *ChannelTopologyReconciler) topologyStatusBuildingWithSpecify(topology *datachannelv1alpha1.ChannelTopology,
	request intctrlutil.RequestCtx, status datachannelv1alpha1.ChannelTopologyStatus) error {
	statusPatch := client.MergeFrom(topology.DeepCopy())
	topology.Status = status
	return r.Client.Status().Patch(request.Ctx, topology, statusPatch)
}

func (r *ChannelTopologyReconciler) findChannelDef(
	request intctrlutil.RequestCtx,
	channelDefName string,
	clusterDefName string,
) (datachannelv1alpha1.ChannelDefinition, error) {
	channelDef := datachannelv1alpha1.ChannelDefinition{}

	if channelDefName == "" && clusterDefName == "" {
		return channelDef, nil
	}

	var err error

	if channelDefName != "" {
		namespacedName := types.NamespacedName{
			Namespace: request.Req.Namespace,
			Name:      channelDefName,
		}
		err = r.Client.Get(request.Ctx, namespacedName, &channelDef)
		return channelDef, err
	} else {
		channelDefs := &datachannelv1alpha1.ChannelDefinitionList{}
		if err = r.Client.List(request.Ctx, channelDefs,
			client.InNamespace(request.Req.Namespace),
			client.MatchingLabels(buildChannelDefLabels(clusterDefName))); err != nil {
			return channelDef, err
		} else {
			for _, chd := range channelDefs.Items {
				if chd.Spec.IsDefault {
					channelDef = chd
					break
				}
			}
		}
	}

	return channelDef, nil
}

func (r *ChannelTopologyReconciler) findChannelTopologyByObject() *handler.Funcs {
	return &handler.Funcs{
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			labels := e.ObjectNew.GetLabels()

			if _, ok := labels[constant.ChannelTopologyLabelKey]; ok {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: e.ObjectNew.GetNamespace(),
					Name:      labels[constant.ChannelTopologyLabelKey],
				}})
			}
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChannelTopologyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&datachannelv1alpha1.ChannelTopology{}).
		Watches(&source.Kind{Type: &appv1alpha1.OpsRequest{}}, r.findChannelTopologyByObject()).
		Watches(&source.Kind{Type: &appv1alpha1.Cluster{}}, r.findChannelTopologyByObject())

	return b.Complete(r)
}
