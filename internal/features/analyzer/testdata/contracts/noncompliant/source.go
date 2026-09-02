package noncompliant

import _ "unsafe"

// analyzer-contract: device-source-compatibility negative
//
//go:linkname unavailableExit runtime.Goexit
func unavailableExit()
