package compliant

// analyzer-contract: device-source-compatibility positive
// analyzer-contract: workspace-static-contract positive
func portableIdentity[T any](value T) T { return value }
