package doctor

import (
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/toolchainprofile"
)

func matchesVerifiedTool(name, version string) bool {
	profile := toolchainprofile.Accepted()
	switch name {
	case "go":
		return strings.Contains(version, "go version go"+profile.Go+" ")
	case "tinygo":
		return strings.Contains(version, "tinygo version "+profile.TinyGo+" ")
	case "arm-none-eabi-gcc":
		return strings.Contains(version, ") "+profile.ArmGCC+" ") || strings.HasSuffix(version, ") "+profile.ArmGCC)
	default:
		return false
	}
}

func detectedToolchainSummary(report Report) string {
	var parts []string
	for _, name := range []string{"tinygo", "arm-none-eabi-gcc"} {
		if tool, ok := report.tool(name); ok && tool.Version != "" {
			parts = append(parts, tool.Version)
		}
	}
	return strings.Join(parts, "; ")
}
