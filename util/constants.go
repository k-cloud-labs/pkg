package util

// Define annotations used by k-cloud-labs
const (
	// AppliedOverrides is the annotation which used to record override items an object applied.
	// The overrides items should be sorted alphabetically in ascending order by OverridePolicy's name.
	AppliedOverrides = "policy.kcloudlabs.io/applied-overrides"

	// AppliedClusterOverrides is the annotation which used to record override items an object applied.
	// The overrides items should be sorted alphabetically in ascending order by ClusterOverridePolicy's name.
	AppliedClusterOverrides = "policy.kcloudlabs.io/applied-cluster-overrides"
)

// Define supported mutate operation
const (
	// Create operation
	Create = "Create"

	// Update operation
	Update = "Update"
)

// Define resource filed
const (
	// SpecField indicates the 'spec' field of a resource
	SpecField = "spec"
)
