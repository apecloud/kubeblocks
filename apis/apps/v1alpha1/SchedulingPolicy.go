package v1alpha1

import corev1 "k8s.io/api/core/v1"

// SchedulingPolicy the scheduling policy.
type SchedulingPolicy struct {
	// If specified, the Pod will be dispatched by specified scheduler.
	// If not specified, the Pod will be dispatched by default scheduler.
	//
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// NodeSelector is a selector which must be true for the Pod to fit on a node.
	// Selector which must match a node's labels for the Pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	//
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// NodeName is a request to schedule this Pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this Pod onto that node, assuming that it fits resource
	// requirements.
	//
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Specifies a group of affinity scheduling rules of the Cluster, including NodeAffinity, PodAffinity, and PodAntiAffinity.
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Allows Pods to be scheduled onto nodes with matching taints.
	// Each toleration in the array allows the Pod to tolerate node taints based on
	// specified `key`, `value`, `effect`, and `operator`.
	//
	// - The `key`, `value`, and `effect` identify the taint that the toleration matches.
	// - The `operator` determines how the toleration matches the taint.
	//
	// Pods with matching tolerations are allowed to be scheduled on tainted nodes, typically reserved for specific purposes.
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints describes how a group of Pods ought to spread across topology
	// domains. Scheduler will schedule Pods in a way which abides by the constraints.
	// All topologySpreadConstraints are ANDed.
	//
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}
