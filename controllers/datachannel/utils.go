package datachannel

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	datachannelv1alpha1 "github.com/apecloud/kubeblocks/apis/datachannel/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	kbcontrollerutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ChannelTopologyResources struct {
	ChannelTopology      *datachannelv1alpha1.ChannelTopology
	ChannelResources     map[string]ChannelResources
	ChannelBuildingPlans []ChannelBuildingPlan
}

type ChannelResources struct {
	ChannelName         string
	ChannelTopologyName string
	ChannelSyncObjs     []datachannelv1alpha1.SyncMetaObject

	SourceChannelDefName string
	SinkChannelDefName   string

	SourceNodeCluster *appv1alpha1.Cluster
	SinkNodeCluster   *appv1alpha1.Cluster

	SourceChannelDefinition  *datachannelv1alpha1.ChannelDefinition
	SinkChannelDefinition    *datachannelv1alpha1.ChannelDefinition
	RelyHubChannelDefinition map[string]*datachannelv1alpha1.ChannelDefinition
}

type ChannelBuildingPlan struct {
	ChannelImpl      ChannelResources
	ChannelResources ChannelResources
	isBuildingSource bool // true means building source, false means building sink

	BuildingCluster appv1alpha1.Cluster
	ExtraEnvs       map[string]string
}

type ChannelTopologyHandler struct {
	Resources *ChannelTopologyResources
	Request   kbcontrollerutil.RequestCtx
	Result    reconcile.Result
	Err       error
}

func (c *ChannelTopologyHandler) findEmptyResources() []string {
	emptyClusterNames := make([]string, 0)

	checkAndBuildResult := func(clusterName string) {
		if clusterName == "" {
			emptyClusterNames = append(emptyClusterNames, clusterName)
		}
	}

	// Todo: check with hub
	for _, channel := range c.Resources.ChannelResources {
		checkAndBuildResult(channel.SourceNodeCluster.Name)
		checkAndBuildResult(channel.SinkNodeCluster.Name)
	}

	return emptyClusterNames
}

func (c *ChannelTopologyHandler) findStatusUnMatchResources(targetStatusPhase appv1alpha1.ClusterPhase) []string {
	unMatchClusterNames := make([]string, 0)

	checkAndBuildResult := func(clusterName string, statusPhase appv1alpha1.ClusterPhase) {
		if statusPhase != targetStatusPhase {
			unMatchClusterNames = append(unMatchClusterNames, clusterName)
		}
	}

	// Todo: check with hub
	for _, channel := range c.Resources.ChannelResources {
		checkAndBuildResult(channel.SourceNodeCluster.Name, channel.SourceNodeCluster.Status.Phase)
		checkAndBuildResult(channel.SinkNodeCluster.Name, channel.SinkNodeCluster.Status.Phase)
	}

	return unMatchClusterNames
}

func (cp *ChannelBuildingPlan) buildingInit(topology *datachannelv1alpha1.ChannelTopology) *ChannelBuildingPlan {
	var clusterSpec *appv1alpha1.ClusterSpec
	var suffix string
	if cp.isBuildingSource {
		clusterSpec = cp.ChannelResources.SourceChannelDefinition.Spec.Source.KubeBlocks.Worker
		suffix = "source"
	} else {
		clusterSpec = cp.ChannelResources.SinkChannelDefinition.Spec.Sink.KubeBlocks.Worker
		suffix = "sink"
	}

	// override channel definition with topology settings.
	clusterSpec.Affinity = topology.Spec.Settings.Schedule.Affinity
	clusterSpec.Tolerations = topology.Spec.Settings.Schedule.Tolerations

	cp.BuildingCluster = appv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s-%s", topology.Name, cp.ChannelResources.ChannelName, suffix),
			Namespace:   topology.Namespace,
			Labels:      buildChannelTopologyLabels(topology),
			Annotations: buildChannelAnnotations(&cp.ChannelResources, cp.isBuildingSource),
		},
		Spec: *clusterSpec,
	}

	channelType := datachannelv1alpha1.SourceChannelType
	if !cp.isBuildingSource {
		channelType = datachannelv1alpha1.SinkChannelType
	}
	cp.buildExtraEnvMap(ExtraEnvChannelType, string(channelType))

	return cp
}

func (cp *ChannelBuildingPlan) buildOwnerReference(topology *datachannelv1alpha1.ChannelTopology, request kbcontrollerutil.RequestCtx) *ChannelBuildingPlan {
	scheme, _ := datachannelv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(topology, &cp.BuildingCluster, scheme); err != nil {
		request.Log.Error(err, err.Error())
	}
	return cp
}

func (cp *ChannelBuildingPlan) buildingWithRelyClusters() *ChannelBuildingPlan {
	// building service endpoint
	buildServiceEndpointVal := func(cluster *appv1alpha1.Cluster, channelDef *datachannelv1alpha1.ChannelDefinition) (string, string) {
		if channelDef.Spec.KubeBlocksSettings.Expose.ComponentDefRef == "" ||
			channelDef.Spec.KubeBlocksSettings.Expose.Service.Port == 0 {
			return "", ""
		}
		return fmt.Sprintf("%s-%s",
				cluster.Name, channelDef.Spec.KubeBlocksSettings.Expose.ComponentDefRef),
			strconv.Itoa(int(channelDef.Spec.KubeBlocksSettings.Expose.Service.Port))
	}

	sourceResource := cp.ChannelResources
	if cp.ChannelImpl.SourceNodeCluster != nil && !isClientObjectEmpty(cp.ChannelImpl.SourceNodeCluster) {
		sourceResource = cp.ChannelImpl
	}
	sourceHost, sourcePort := buildServiceEndpointVal(sourceResource.SourceNodeCluster, sourceResource.SourceChannelDefinition)
	cp.buildExtraEnvMap(ExtraEnvSourceHostname, sourceHost)
	cp.buildExtraEnvMap(ExtraEnvSourcePort, sourcePort)

	sinkHost, sinkPort := buildServiceEndpointVal(cp.ChannelResources.SinkNodeCluster, cp.ChannelResources.SinkChannelDefinition)
	cp.buildExtraEnvMap(ExtraEnvSinkHostname, sinkHost)
	cp.buildExtraEnvMap(ExtraEnvSinkPort, sinkPort)

	return cp
}

func (cp *ChannelBuildingPlan) buildingWithAccount(client client.Client, request kbcontrollerutil.RequestCtx) *ChannelBuildingPlan {
	// Todo: rely hub support account
	// Todo: to consider security
	buildAccountVal := func(cluster *appv1alpha1.Cluster, channelDef *datachannelv1alpha1.ChannelDefinition, isSource bool) (string, string) {
		accountDef := channelDef.Spec.Source.KubeBlocks.AccountRequests
		if !isSource {
			accountDef = channelDef.Spec.Sink.KubeBlocks.AccountRequests
		}

		if accountDef.ComponentName == "" || accountDef.AccountName == "" {
			return "", ""
		}
		accountSecret := &v1.Secret{}
		namespacedName := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("%s-%s-%s", cluster.Name, accountDef.ComponentName, accountDef.AccountName),
		}

		var err error
		accountName, accountPassword := "", ""
		if err = client.Get(request.Ctx, namespacedName, accountSecret); err != nil {
			request.Log.Error(err, err.Error())
			return "", ""
		}

		// Todo: why base64 decode meet error ?
		accountName, err = plan.DecodeString(accountSecret.Data["username"])
		if err != nil {
			request.Log.Error(err, err.Error())
			accountName = string(accountSecret.Data["username"])
		}
		accountPassword, err = plan.DecodeString(accountSecret.Data["password"])
		if err != nil {
			request.Log.Error(err, err.Error())
			accountPassword = string(accountSecret.Data["password"])
		}

		return accountName, accountPassword
	}

	sourceUser, sourcePassword := buildAccountVal(cp.ChannelResources.SourceNodeCluster, cp.ChannelResources.SourceChannelDefinition, true)
	cp.buildExtraEnvMap(ExtraEnvSourceUser, sourceUser)
	cp.buildExtraEnvMap(ExtraEnvSourcePassword, sourcePassword)

	sinkUser, sinkPassword := buildAccountVal(cp.ChannelResources.SinkNodeCluster, cp.ChannelResources.SinkChannelDefinition, false)
	cp.buildExtraEnvMap(ExtraEnvSinkUser, sinkUser)
	cp.buildExtraEnvMap(ExtraEnvSinkPassword, sinkPassword)

	return cp
}

func (cp *ChannelBuildingPlan) buildingWithSyncObjs() *ChannelBuildingPlan {
	syncObjExpressArr := cp.ChannelResources.SourceChannelDefinition.Spec.Source.SyncObjEnvExpress
	if !cp.isBuildingSource {
		syncObjExpressArr = cp.ChannelResources.SinkChannelDefinition.Spec.Sink.SyncObjEnvExpress
	}

	if len(syncObjExpressArr) == 0 {
		return cp
	}

	for _, express := range syncObjExpressArr {
		if express.IsEmpty() {
			continue
		}
		cp.convertArgs(&express)
		if expressStr := transformToChannelDefExpressWithRootObj(express,
			cp.ChannelResources.ChannelSyncObjs, cp.isBuildingSource); expressStr != "" {
			cp.buildExtraEnvMap(generateEnvName(fmt.Sprintf(ExtraEnvUdf, express.Name)), expressStr)
		}
	}

	return cp
}

func (cp *ChannelBuildingPlan) buildingWithExtraEnvs() *ChannelBuildingPlan {
	extraEnvMap := cp.ChannelResources.SourceChannelDefinition.Spec.Source.KubeBlocks.ExtraEnvs
	if !cp.isBuildingSource {
		extraEnvMap = cp.ChannelResources.SinkChannelDefinition.Spec.Sink.KubeBlocks.ExtraEnvs
	}

	// basic param
	cp.buildExtraEnvMap(ExtraEnvChannelTopologyName, cp.ChannelResources.ChannelTopologyName)
	cp.buildExtraEnvMap(ExtraEnvChannelName, cp.ChannelResources.ChannelName)

	rand.Seed(time.Now().UnixNano())
	// Todo: mock the minNum=1 for mysql serverId.
	cp.buildExtraEnvMap(ExtraEnvChannelRandomInt16, strconv.Itoa(int(rand.Int63nRange(1, 1<<16))))

	for extraKey, extraValue := range extraEnvMap {
		newValue, _ := convertWithSystemVars(extraValue, cp.ExtraEnvs)
		cp.buildExtraEnvMap(generateEnvName(extraKey), newValue)
	}

	return cp
}

func (cp *ChannelBuildingPlan) convertArgs(express *datachannelv1alpha1.SyncObjEnvExpress) error {
	systemVars := map[string]string{
		ChannelNameArg:         cp.ChannelResources.ChannelName,
		ChannelTopologyNameArg: cp.ChannelResources.ChannelTopologyName,
	}

	prefixStr, err := convertWithSystemVars(express.Prefix, systemVars)
	if err != nil {
		return err
	} else {
		express.Prefix = prefixStr
	}

	suffixStr, err := convertWithSystemVars(express.Suffix, systemVars)
	if err != nil {
		return err
	} else {
		express.Suffix = suffixStr
	}

	return nil
}

func (cp *ChannelBuildingPlan) transform() error {
	if len(cp.ExtraEnvs) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(cp.ExtraEnvs)
	if err == nil {
		cp.BuildingCluster.Annotations[constant.ExtraEnvAnnotationKey] = string(jsonData)
	}
	return err
}

func (cp *ChannelBuildingPlan) buildExtraEnvMap(envKey string, envValue string) {
	if envKey == "" || envValue == "" {
		return
	}
	envKey = strings.ToUpper(envKey)
	if cp.ExtraEnvs == nil {
		cp.ExtraEnvs = map[string]string{
			envKey: envValue,
		}
	} else {
		cp.ExtraEnvs[envKey] = envValue
	}
}

// TopologyNodeEdge drag mode impl from internal/controller/graph/dag.go
type TopologyNodeEdge struct {
	Source string
	Sink   string
}

func (e TopologyNodeEdge) From() graph.Vertex {
	return e.Source
}

func (e TopologyNodeEdge) To() graph.Vertex {
	return e.Sink
}

// validateTopologyStruct: check if the channel definition is validated with the topologyStruct setting.
// return isValid bool result and an error message.
func validateTopologyStruct(channels []datachannelv1alpha1.ChannelDefine, topologyStruct datachannelv1alpha1.TopologyStruct) (bool, string) {
	if topologyStruct != datachannelv1alpha1.DAGTopologyStruct {
		return true, ""
	}

	dag := graph.NewDAG()
	for _, channel := range channels {
		sourceSymbol, sinkSymbol := buildSymbolWithChannelNodeDefine(channel.From), buildSymbolWithChannelNodeDefine(channel.To)

		dag.AddVertex(sourceSymbol)
		dag.AddVertex(sinkSymbol)
		dag.AddEdge(TopologyNodeEdge{
			Source: sourceSymbol,
			Sink:   sinkSymbol,
		})
	}

	if err := dag.Validate(); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func isContainAll(base map[string]string, target map[string]string) bool {
	if len(base) == 0 {
		return false
	}
	if len(target) == 0 {
		return true
	}

	containCount := 0

	for k, v := range target {
		if baseV, contain := base[k]; contain && baseV == v {
			containCount++
		}
	}

	return containCount == len(target)
}

func isOpsAnnotationsMatch(annotations map[string]string, channelName string) bool {
	targetAnnotationMap := map[string]string{
		constant.ChannelNameAnnotationKey: channelName,
	}

	return isContainAll(annotations, targetAnnotationMap)
}

func isClusterAnnotationsMatch(annotations map[string]string, channelName string, isSource bool) bool {
	isMatch := false

	if len(annotations) == 0 || channelName == "" {
		return isMatch
	}

	targetAnnotationKey := constant.ChannelSourceRelationAnnotationKey
	if !isSource {
		targetAnnotationKey = constant.ChannelSinkRelationAnnotationKey
	}

	if annotationValue, ok := annotations[targetAnnotationKey]; ok {
		if annotationValue == "" {
			return isMatch
		}
		for _, name := range strings.Split(annotationValue, ",") {
			if channelName == name {
				isMatch = true
				break
			}
		}
	}

	return isMatch
}

func isChannelClusterAnnotationsMatch(annotations map[string]string, channelName string, isSource bool) bool {
	isMatch := false

	if len(annotations) == 0 || channelName == "" {
		return isMatch
	}

	clusterType := datachannelv1alpha1.SourceChannelType
	if !isSource {
		clusterType = datachannelv1alpha1.SinkChannelType
	}
	targetAnnotations := map[string]string{
		constant.ChannelNameAnnotationKey: channelName,
		constant.ChannelTypeAnnotationKey: string(clusterType),
	}

	return isContainAll(annotations, targetAnnotations)
}

// buildChannelTopologyWithDefaultSettings: generate default settings for a ChannelTopology obj.
func buildChannelTopologyWithDefaultSettings(channelTopology *datachannelv1alpha1.ChannelTopology) {
	if channelTopology.Spec.Settings.Topology.PrepareTTLMinutes == 0 {
		channelTopology.Spec.Settings.Topology.PrepareTTLMinutes = datachannelv1alpha1.DefaultPrepareTTLMinutes
	}
	if channelTopology.Spec.Settings.Topology.TopologyStruct == "" {
		channelTopology.Spec.Settings.Topology.TopologyStruct = datachannelv1alpha1.DAGTopologyStruct
	}
	if channelTopology.Spec.Settings.Topology.BuildingPolicy == "" {
		channelTopology.Spec.Settings.Topology.BuildingPolicy = datachannelv1alpha1.ClusterPriorityBuildingPolicy
	}
}

func buildChannelTopologyLabels(channelTopology *datachannelv1alpha1.ChannelTopology) map[string]string {
	return map[string]string{
		constant.ChannelTopologyLabelKey: channelTopology.Name,
	}
}

func buildClusterNameLabels(clusterName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey: clusterName,
	}
}

func buildChannelDefLabels(clusterDefName string) map[string]string {
	return map[string]string{
		constant.ClusterDefLabelKey: clusterDefName,
	}
}

func buildChannelAnnotations(channelResource *ChannelResources, isSource bool) map[string]string {
	channelType := datachannelv1alpha1.SourceChannelType
	if !isSource {
		channelType = datachannelv1alpha1.SinkChannelType
	}
	return map[string]string{
		constant.ChannelNameAnnotationKey: channelResource.ChannelName,
		constant.ChannelTypeAnnotationKey: string(channelType),
	}
}

func buildClusterOpsAnnotations(channelName string) map[string]string {
	return map[string]string{
		constant.ChannelNameAnnotationKey: channelName,
	}
}

func buildOpsRequestForChannel(channelTopology *datachannelv1alpha1.ChannelTopology, channelResource *ChannelResources) *appv1alpha1.OpsRequest {
	return &appv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: channelResource.SourceNodeCluster.Name + "-",
			Namespace:    channelResource.SourceNodeCluster.Namespace,
			Labels:       buildChannelTopologyLabels(channelTopology),
			Annotations:  buildClusterOpsAnnotations(channelResource.ChannelName),
		},
		Spec: appv1alpha1.OpsRequestSpec{
			Type:        appv1alpha1.ReconfiguringType,
			ClusterRef:  channelResource.SourceNodeCluster.Name,
			Reconfigure: channelResource.SourceChannelDefinition.Spec.Source.KubeBlocks.ConfigureRequests,
		},
	}
}

func isClientObjectEmpty(obj client.Object) bool {
	if obj == nil {
		return true
	}
	return obj.GetName() == ""
}

func generateEnvName(envKey string) string {
	return strings.ToUpper(strings.ReplaceAll(envKey, "-", "_"))
}

// transformToChannelDefExpressWithRootObj use transformToChannelDefExpress to transform express per syncObject
func transformToChannelDefExpressWithRootObj(se datachannelv1alpha1.SyncObjEnvExpress,
	objs []datachannelv1alpha1.SyncMetaObject,
	isSource bool,
) string {
	if se.IsEmpty() || len(objs) == 0 {
		return ""
	}
	resultArr := make([]string, 0)

	for _, obj := range objs {
		if str := transformToChannelDefExpress(se, se.MetaTypeRequired, obj, nil, isSource); str != "" {
			resultArr = append(resultArr, str)
		}
	}

	return strings.Join(resultArr, se.MetaObjConnectSymbol)
}

// transformToChannelDefExpress Return non-empty only when the deepest node is reached and the deepest node isAll=true
func transformToChannelDefExpress(se datachannelv1alpha1.SyncObjEnvExpress,
	metaTypeRequired []datachannelv1alpha1.SyncMetaType,
	so datachannelv1alpha1.SyncMetaObject,
	relativePathArr []string,
	isSource bool,
) string {
	if relativePathArr == nil {
		relativePathArr = make([]string, 0)
	}

	if se.IsEmpty() || so.Name == "" || len(metaTypeRequired) == 0 {
		return strings.Join(relativePathArr, se.MetaTypeConnectSymbol)
	}

	wrapWithSuffixAndPrefix := func(targetStr string) string {
		if targetStr == "" {
			return ""
		}
		return fmt.Sprintf("%s%s%s", se.Prefix, targetStr, se.Suffix)
	}

	workWithObjName := func() string {
		if !isSource && so.MappingName != "" {
			return so.MappingName
		} else {
			return so.Name
		}
	}

	matchIdx := -1
	for index, t := range metaTypeRequired {
		if t == so.Type {
			matchIdx = index
			break
		}
	}

	// if object-type matched with metaTypeRequired and depth of metaTypeRequired == 1
	if matchIdx > -1 && len(metaTypeRequired) == 1 {
		// is isAll=true, and required-meta is the last one, append result and return.
		switch se.SelectMode {
		case datachannelv1alpha1.InvolvedSelectMode:
			relativePathArr = append(relativePathArr, workWithObjName())
			return wrapWithSuffixAndPrefix(strings.Join(relativePathArr, se.MetaTypeConnectSymbol))
		default:
			// selectMode = ExactlySelectMode;"";something else
			if so.IsAll {
				relativePathArr = append(relativePathArr, workWithObjName())
				return wrapWithSuffixAndPrefix(strings.Join(relativePathArr, se.MetaTypeConnectSymbol))
			}
			return ""
		}

	}
	// object-type is not match or depth of metaTypeRequired > 1
	if len(so.Child) > 0 {
		// recursion when object has child.
		resultStr := make([]string, 0)
		subMetaTypeRequired := metaTypeRequired
		if matchIdx > -1 {
			relativePathArr = append(relativePathArr, workWithObjName())
			if matchIdx < len(metaTypeRequired) {
				subMetaTypeRequired = make([]datachannelv1alpha1.SyncMetaType, len(metaTypeRequired)-matchIdx-1)
				copy(subMetaTypeRequired, metaTypeRequired[matchIdx+1:])
			}
		}
		for _, child := range so.Child {
			if str := transformToChannelDefExpress(se, subMetaTypeRequired, child, relativePathArr, isSource); str != "" {
				resultStr = append(resultStr, str)
			}
		}
		return strings.Join(resultStr, se.MetaObjConnectSymbol)
	}

	// find no path, return empty.
	return ""
}

func convertWithSystemVars(baseStr string, systemVars map[string]string) (string, error) {
	if baseStr == "" || len(systemVars) == 0 {
		return baseStr, nil
	}

	var err error
	var temp *template.Template

	for varKey, _ := range systemVars {
		baseStr = strings.ReplaceAll(baseStr, fmt.Sprintf("${%s}", varKey), fmt.Sprintf("{{.%s}}", varKey))
	}

	temp, err = template.New("").Parse(baseStr)
	if err != nil {
		return "", err
	}

	s := new(strings.Builder)
	err = temp.Execute(s, systemVars)

	return s.String(), err
}

func buildSymbolWithChannelNodeDefine(channelNode datachannelv1alpha1.ChannelNodeDefine) string {
	if channelNode.ClusterRef != "" && channelNode.ClusterNamespace != "" {
		return fmt.Sprintf("%s-%s", channelNode.ClusterRef, channelNode.ClusterNamespace)
	}
	if channelNode.HubRef != "" {
		return channelNode.HubRef
	}
	return ""
}
