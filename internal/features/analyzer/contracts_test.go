package analyzer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestContractInventoryValid(t *testing.T) {
	inventory := ContractInventory()
	if err := inventory.Validate(); err != nil {
		t.Fatal(err)
	}
	seenKinds := make(map[ContractKind]bool)
	for _, contract := range inventory.Contracts {
		seenKinds[contract.Kind] = true
		if !validDocumentationReference(contract.NormativeRef) {
			t.Errorf("contract %q has invalid normative reference %q", contract.ID, contract.NormativeRef)
		}
	}
	for _, kind := range []ContractKind{ContractReplacement, ContractOptionalCapability, ContractCallbackScope, ContractOwnedHandle, ContractBorrowedHandle, ContractRetention, ContractClose, ContractUpdateOnly, ContractArgumentBound, ContractAvailability} {
		if !seenKinds[kind] {
			t.Errorf("inventory has no %q contract", kind)
		}
	}
}

func TestInventoryJSONDeterministicAndRoundTrips(t *testing.T) {
	first, err := ContractInventory().JSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := ContractInventory().JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("inventory JSON is not deterministic")
	}
	if !strings.HasSuffix(string(first), "\n") {
		t.Fatal("inventory JSON has no trailing newline")
	}
	var decoded Inventory
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded inventory: %v", err)
	}
}

func TestInventoryValidationRejectsUnknownContract(t *testing.T) {
	inventory := ContractInventory()
	inventory.Rules[0].ContractIDs = []string{"missing-contract"}
	if err := inventory.Validate(); err == nil || !strings.Contains(err.Error(), "unknown contract") {
		t.Fatalf("Validate() = %v, want unknown contract error", err)
	}
}

func TestInventoryReturnsIndependentSlices(t *testing.T) {
	first := ContractInventory()
	first.Contracts[0].ID = "changed-contract"
	first.Rules[0].ID = "changed-rule"
	first.RuntimeMeasurements[0] = "changed"
	second := ContractInventory()
	if second.Contracts[0].ID == "changed-contract" || second.Rules[0].ID == "changed-rule" || second.RuntimeMeasurements[0] == "changed" {
		t.Fatal("ContractInventory returned shared mutable slices")
	}
}
