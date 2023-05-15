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

package fault

// Unchanged DryRun flag
const (
	Unchanged = "unchanged"
)

// GVR
const (
	Group   = "chaos-mesh.org"
	Version = "v1alpha1"

	ResourcePodChaos     = "podchaos"
	ResourceNetworkChaos = "networkchaos"
	ResourceIOChaos      = "iochaos"
	ResourceStressChaos  = "stresschaos"
	ResourceDNSChaos     = "dnschaos"
	ResourceTimeChaos    = "timechaos"
	ResourceHTTPChaos    = "httpchaos"
	ResourceAWSChaos     = "awschaos"
	ResourceGCPChaos     = "gcpchaos"
)

// Cue Template Name
const (
	CueTemplatePodChaos     = "pod_chaos_template.cue"
	CueTemplateNetworkChaos = "network_chaos_template.cue"
	CueTemplateIOChaos      = "io_chaos_template.cue"
	CueTemplateStressChaos  = "stress_chaos_template.cue"
	CueTemplateDNSChaos     = "dns_chaos_template.cue"
	CueTemplateTimeChaos    = "time_chaos_template.cue"
	CueTemplateHTTPChaos    = "http_chaos_template.cue"
	CueTemplateAWSChaos     = "aws_chaos_template.cue"
	CueTemplateGCPChaos     = "gcp_chaos_template.cue"
)

// Pod Chaos Command
const (
	Kill               = "kill"
	KillShort          = "kill pod"
	Failure            = "failure"
	FailureShort       = "failure pod"
	KillContainer      = "kill-container"
	KillContainerShort = "kill containers"
)

// NetWork Chaos Command
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

// DNS Chaos Command
const (
	Random      = "random"
	RandomShort = "Make DNS return any IP when resolving external domain names."
	Error       = "error"
	ErrorShort  = "Make DNS return an error when resolving external domain names."
)

// Network Chaos Command
const (
	Latency        = "latency"
	LatencyShort   = "Delayed IO operations."
	Fault          = "fault"
	FaultShort     = "Causes IO operations to return specific errors."
	Attribute      = "attribute"
	AttributeShort = "Override the attributes of the file."
	Mistake        = "mistake"
	MistakeShort   = "Alters the contents of the file, distorting the contents of the file."
)

// Stress Chaos Command
const (
	Stress      = "stress"
	StressShort = "Add memory pressure or CPU load to the system."
)

// Time Chaos Command
const (
	Time      = "time"
	TimeShort = "Clock skew failure."
)

// HTTP Chaos Command
const (
	Abort          = "abort"
	AbortShort     = "Abort the HTTP request and response."
	HTTPDelay      = "delay"
	HTTPDelayShort = "Delay the HTTP request and response."
	Replace        = "replace"
	ReplaceShort   = "Replace the HTTP request and response."
	Patch          = "patch"
	PatchShort     = "Patch the HTTP request and response."
)

// AWS And GCP Chaos Command
const (
	Stop              = "stop"
	StopShort         = "Stop instance"
	Restart           = "restart"
	RestartShort      = "Restart instance"
	DetachVolume      = "detach-volume"
	DetachVolumeShort = "Detach volume"
)
