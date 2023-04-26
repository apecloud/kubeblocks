package fault

const (
	Group   = "chaos-mesh.org"
	Version = "v1alpha1"

	ResourcePodChaos     = "podchaos"
	ResourceNetworkChaos = "networkchaos"
)

const (
	CueTemplatePodChaosName     = "podChaos_template.cue"
	CueTemplateNetworkChaosName = "podChaos_template.cue"
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
