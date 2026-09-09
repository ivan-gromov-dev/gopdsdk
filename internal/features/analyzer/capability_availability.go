package analyzer

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/analysis"
)

type capabilityAvailability struct {
	Package          string
	Name             string
	GopdsdkSince     string
	PlaydateSDKSince string
	Targets          []Target
	Rule             RuleID
}

func capabilityAvailabilityTable() []capabilityAvailability {
	for _, contract := range ContractInventory().Contracts {
		if contract.ID == "video-capability-availability" && len(contract.PublicAPI) == 1 {
			symbol := contract.PublicAPI[0]
			return []capabilityAvailability{{Package: playdatePackage, Name: symbol.Name, GopdsdkSince: symbol.SinceGopdsdk, PlaydateSDKSince: symbol.MinimumPlaydateSDK, Targets: contract.Targets, Rule: "capability-video-availability"}}
		}
	}
	return nil
}

func validateReleaseVersion(label, version string, requireV bool) error {
	normalized := version
	prefix := ""
	if !requireV {
		normalized = "v" + version
	} else {
		prefix = "v"
	}
	if version == "" || (requireV && !strings.HasPrefix(version, "v")) || !semver.IsValid(normalized) || semver.Prerelease(normalized) != "" || semver.Build(normalized) != "" {
		return fmt.Errorf("%s %q must be a stable %sMAJOR.MINOR.PATCH release", label, version, prefix)
	}
	return nil
}

func capabilityAvailabilityFindings(snapshot Snapshot, gopdsdkFloor, playdateSDK string) []Finding {
	var findings []Finding
	availabilityTable := capabilityAvailabilityTable()
	for _, loaded := range snapshot.Loaded {
		for identifier, object := range loaded.TypesInfo.Uses {
			typeName, ok := object.(*types.TypeName)
			if !ok || typeName.Pkg() == nil {
				continue
			}
			for _, availability := range availabilityTable {
				if typeName.Pkg().Path() != availability.Package || typeName.Name() != availability.Name || !containsTarget(availability.Targets, snapshot.Target) {
					continue
				}
				var reasons []string
				if gopdsdkFloor != "" && semver.Compare(gopdsdkFloor, availability.GopdsdkSince) < 0 {
					reasons = append(reasons, "declared gopdsdk floor "+gopdsdkFloor+" predates "+availability.GopdsdkSince)
				}
				if playdateSDK != "" && semver.Compare("v"+playdateSDK, "v"+availability.PlaydateSDKSince) < 0 {
					reasons = append(reasons, "configured Playdate SDK "+playdateSDK+" predates "+availability.PlaydateSDKSince)
				}
				if len(reasons) != 0 {
					diagnostic := analysis.Diagnostic{Pos: identifier.Pos(), End: identifier.End(), Message: "video capability is unavailable: " + strings.Join(reasons, "; ") + "; raise the floor or gate the feature"}
					findings = append(findings, normalizeDiagnostic(snapshot, loaded, availability.Rule, "capabilityavailability", diagnostic))
				}
			}
		}
	}
	return findings
}
