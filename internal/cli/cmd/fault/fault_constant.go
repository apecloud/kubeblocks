package fault

const (
	Unchanged = "unchanged"
)

const (
	Group   = "chaos-mesh.org"
	Version = "v1alpha1"

	ResourcePodChaos     = "podchaos"
	ResourceNetworkChaos = "networkchaos"
	ResourceIOChaos      = "iochaos"
	ResourceStressChaos  = "stresschaos"
	ResourceDNSChaos     = "dnschaos"
	ResourceTimeChaos    = "timechaos"
)

const (
	CueTemplatePodChaos     = "podChaos_template.cue"
	CueTemplateNetworkChaos = "networkChaos_template.cue"
	CueTemplateIOChaos      = "IOChaos_template.cue"
	CueTemplateStressChaos  = "stressChaos_template.cue"
	CueTemplateDNSChaos     = "DNSChaos_template.cue"
	CueTemplateTimeChaos    = "timeChaos_template.cue"
)

const (
	Kill               = "kill"
	KillShort          = "kill pod"
	Failure            = "failure"
	FailureShort       = "failure pod"
	KillContainer      = "kill-container"
	KillContainerShort = "kill containers"
)

// NetWorkChaos Command
const (
	Partition      = "partition"
	PartitionShort = "Make a pod network partitioned from other objects."
	Loss           = "loss"
	LossShort      = "Cause pods to communicate with other objects to drop packets."
	Delay          = "delay"
	DelayShort     = "Make pods communicate with other objects lazily."
	Duplicate      = "duplicate"
	DuplicateShort = "Make pods communicate with other objects to pick up duplicate packets."
	Corrupt        = "corrupt"
	CorruptShort   = "Distorts the messages a pod communicates with other objects."
	Bandwidth      = "bandwidth"
	BandwidthShort = "Limit the bandwidth that pods use to communicate with other objects."
)

const (
	Random      = "random"
	RandomShort = "Make DNS return any IP when resolving external domain names."
	Error       = "error"
	ErrorShort  = "Make DNS return an error when resolving external domain names."
)

const (
	Latency        = "latency"
	LatencyShort   = "delayed IO operations."
	Fault          = "fault"
	FaultShort     = "Causes IO operations to return specific errors."
	Attribute      = "attribute"
	AttributeShort = "Override the attributes of the file."
	Mistake        = "mistake"
	MistakeShort   = "Alters the contents of the file, distorting the contents of the file."
)

const (
	Stress      = "stress"
	StressShort = "Add memory pressure or CPU load to the system."
)

const (
	Time      = "time"
	TimeShort = "Clock skew failure."
)
