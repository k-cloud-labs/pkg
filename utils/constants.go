package utils

// Define annotations used by k-cloud-labs
const (
	// AppliedOverrides is the annotation which used to record override items an object applied.
	// The overrides items should be sorted alphabetically in ascending order by OverridePolicy's name.
	AppliedOverrides = "policy.kcloudlabs.io/applied-overrides"

	// AppliedClusterOverrides is the annotation which used to record override items an object applied.
	// The overrides items should be sorted alphabetically in ascending order by ClusterOverridePolicy's name.
	AppliedClusterOverrides = "policy.kcloudlabs.io/applied-cluster-overrides"
)

// Define resource filed
const (
	// SpecField indicates the 'spec' field of a resource
	SpecField = "spec"
)

// Define cue parameter and output name
const (
	// ObjectParameterName is the object parameter name defined in cue
	ObjectParameterName = "object"
	// OldObjectParameterName is the old object parameter name defined in cue, only used with "Update" operation for validate policy
	OldObjectParameterName = "oldObject"
	// DataParameterName is a collection of cue params, including object, oldObject and extraParams.
	DataParameterName = "data"
	// OverrideOutputName is the output name defined in cue for override policy
	OverrideOutputName = "patches"
	// ValidateOutputName is the output name defined in cue for validate policy
	ValidateOutputName = "validate"
)
