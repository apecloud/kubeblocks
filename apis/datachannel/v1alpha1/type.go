package v1alpha1

// define the default settings for ChannelTopology

const (
	DefaultPrepareTTLMinutes int = 15
)

type TopologyStruct string

const (
	DAGTopologyStruct TopologyStruct = "dag"
	DCGTolologyStruct TopologyStruct = "dcg"
)

type BuildingPolicy string

const (
	ClusterPriorityBuildingPolicy BuildingPolicy = "cluster-priority"
	ChannelPriorityBuildingPolicy BuildingPolicy = "channel-priority"
)

type ChannelTopologyPhase string

const (
	PreparingChannelTopologyPhase ChannelTopologyPhase = "Preparing"
	RunningChannelTopologyPhase   ChannelTopologyPhase = "Running"
	FailedChannelTopologyPhase    ChannelTopologyPhase = "Failed"
)

type ChannelType string

const (
	SourceChannelType ChannelType = "source"
	SinkChannelType   ChannelType = "sink"
)

type SyncMetaType string

const (
	DatabaseSyncMeta SyncMetaType = "Database"
	SchemaSyncMeta   SyncMetaType = "Schema"
	TableSyncMeta    SyncMetaType = "Table"
)

type SelectMode string

const (
	ExactlySelectMode  = "Exactly"
	InvolvedSelectMode = "Involved"
)

type ChannelDefWorkerType string

const (
	KubeBlocksWorkerType ChannelDefWorkerType = "KubeBlocks"
)
