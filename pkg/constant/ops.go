package constant

// labels
const (
	OpsRequestTypeLabelKey      = "operations.kubeblocks.io/ops-type"
	OpsRequestNameLabelKey      = "operations.kubeblocks.io/ops-name"
	OpsRequestNamespaceLabelKey = "operations.kubeblocks.io/ops-namespace"
)

// annotations
const (
	RelatedOpsAnnotationKey            = "operations.kubeblocks.io/related-ops"
	OpsDependentOnSuccessfulOpsAnnoKey = "operations.kubeblocks.io/dependent-on-successful-ops"
)
