package fault

const (
	Unchanged = "unchanged"
)

var Options = []string{"PodChaosOptions", "NetworkChaosOptions", "IOChaosOptions"}

const (
	Group   = "chaos-mesh.org"
	Version = "v1alpha1"

	ResourcePodChaos     = "podchaos"
	ResourceNetworkChaos = "networkchaos"
	ResourceIOChaos      = "iochaos"
)

const (
	CueTemplatePodChaos     = "podChaos_template.cue"
	CueTemplateNetworkChaos = "networkChaos_template.cue"
	CueTemplateIOChaos      = "IOChaos_template.cue"
)

const (
	Kill               = "kill"
	KillShort          = "kill a pod"
	Failure            = "failure"
	FailureShort       = "failure a pod"
	KillContainer      = "kill-container"
	KillContainerShort = "kill a container"
)

// NetWorkChaos Command
const (
	Partition      = "partition"
	PartitionShort = "partition attack."
	Loss           = "loss"
	LossShort      = "loss attack"
	Delay          = "delay"
	DelayShort     = "delay attack"
	Duplicate      = "duplicate"
	DuplicateShort = "duplicate attack"
	Corrupt        = "corrupt"
	CorruptShort   = "corrupt attack"
	Bandwidth      = "bandwidth"
	BandwidthShort = "bandwidth attack"
)

const (
	Latency           = "latency"
	LatencyShort      = "IO Latency attack."
	Fault             = "fault"
	FaultShort        = "IO Fault attack."
	AttrOverride      = "attr-override"
	AttrOverrideShort = "IO AttrOverrideShort attack."
)
