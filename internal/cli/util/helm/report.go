package helm

// ReportEntry to store changes between releases
type ReportEntry struct {
	key             string
	suppressedKinds []string
	kind            string
	context         int
	diffs           []difflib.DiffRecord
	changeType      string
}

// ReportTemplateSpec for common template spec
type ReportTemplateSpec struct {
	Namespace string
	Name      string
	Kind      string
	API       string
	Change    string
}
