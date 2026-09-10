package noncompliant

import _ "unsafe"

// analyzer-contract: device-source-compatibility negative
// analyzer-contract: workspace-static-contract negative
//
//go:linkname unavailableExit runtime.Goexit
func unavailableExit()
