package compliant

// analyzer-contract: device-source-compatibility positive
func portableIdentity[T any](value T) T { return value }
