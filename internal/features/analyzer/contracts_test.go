package analyzer

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode"
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

func TestInventoryPublicSymbolsExist(t *testing.T) {
	root := repositoryRoot(t)
	packages := make(map[string]map[string]bool)
	for _, contract := range ContractInventory().Contracts {
		for _, symbol := range contract.PublicAPI {
			if _, loaded := packages[symbol.Package]; !loaded {
				packages[symbol.Package] = exportedSymbols(t, filepath.Join(root, filepath.FromSlash(symbol.Package)))
			}
			key := symbol.Name
			if symbol.Member != "" {
				key += "." + symbol.Member
			}
			if !packages[symbol.Package][key] {
				t.Errorf("contract %q references missing public symbol %s.%s", contract.ID, symbol.Package, key)
			}
		}
	}
}

func TestInventoryDocumentationReferencesExist(t *testing.T) {
	root := repositoryRoot(t)
	loaded := make(map[string]string)
	for _, contract := range ContractInventory().Contracts {
		path, anchor, ok := strings.Cut(contract.NormativeRef, "#")
		if !ok || anchor == "" {
			t.Errorf("contract %q has malformed normative reference %q", contract.ID, contract.NormativeRef)
			continue
		}
		content, exists := loaded[path]
		if !exists {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Errorf("contract %q: read normative document: %v", contract.ID, err)
				continue
			}
			content = string(data)
			loaded[path] = content
		}
		if !markdownHasAnchor(content, anchor) {
			t.Errorf("contract %q references missing anchor %q in %s", contract.ID, anchor, path)
		}
	}
}

func TestEveryPublicCloseContractIsClassified(t *testing.T) {
	public := exportedSymbols(t, filepath.Join(repositoryRoot(t), "playdate"))
	classified := make(map[string]bool)
	for _, contract := range ContractInventory().Contracts {
		if contract.Kind != ContractOwnedHandle && contract.Kind != ContractBorrowedHandle && contract.Kind != ContractClose {
			continue
		}
		for _, symbol := range contract.PublicAPI {
			if symbol.Package == "playdate" && symbol.Member == "Close" {
				classified[symbol.Name+".Close"] = true
			}
		}
	}
	var missing []string
	for symbol := range public {
		if strings.HasSuffix(symbol, ".Close") && !classified[symbol] {
			missing = append(missing, symbol)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		t.Fatalf("public Close contracts missing from analyzer inventory: %s", strings.Join(missing, ", "))
	}
}

func TestCallbackScopeContractsHaveLifetimeRules(t *testing.T) {
	inventory := ContractInventory()
	covered := make(map[string]bool)
	for _, rule := range inventory.Rules {
		if rule.Family != FamilyLifetime {
			continue
		}
		for _, contractID := range rule.ContractIDs {
			covered[contractID] = true
		}
	}
	for _, contract := range inventory.Contracts {
		if contract.Kind == ContractCallbackScope && !covered[contract.ID] {
			t.Errorf("callback-scoped contract %q has no lifetime rule template", contract.ID)
		}
	}
}

func TestDeviceContractsClassifyEveryGoSymbol(t *testing.T) {
	for _, contract := range ContractInventory().Contracts {
		for _, symbol := range contract.GoSymbols {
			if !symbol.Policy.valid() {
				t.Errorf("contract %q leaves %s.%s unclassified", contract.ID, symbol.Package, symbol.Name)
			}
		}
	}
}

func TestOptionalCapabilitiesExistInBothNativeContexts(t *testing.T) {
	root := repositoryRoot(t)
	methods := exportedInterfaceMethods(t, filepath.Join(root, "playdate"))
	templates := []string{
		filepath.Join(root, "internal", "features", "runtime", "simabi", "templates", "simulator.go.tmpl"),
		filepath.Join(root, "internal", "features", "deviceprobe", "templates", "application.go.tmpl"),
	}
	contract := contractByID(t, "context-optional-capability")
	for _, template := range templates {
		data, err := os.ReadFile(template)
		if err != nil {
			t.Fatal(err)
		}
		normalized := strings.Map(func(character rune) rune {
			if unicode.IsSpace(character) {
				return -1
			}
			return character
		}, string(data))
		for _, capability := range contract.PublicAPI {
			if capability.Name == "Context" {
				continue
			}
			for _, method := range methods[capability.Name] {
				needle := "func(playdateContext)" + method + "("
				if !strings.Contains(normalized, needle) {
					t.Errorf("%s does not implement inventoried capability method %s.%s", filepath.Base(template), capability.Name, method)
				}
			}
		}
	}
}

func contractByID(t *testing.T, id string) Contract {
	t.Helper()
	for _, contract := range ContractInventory().Contracts {
		if contract.ID == id {
			return contract
		}
	}
	t.Fatalf("contract %q is missing", id)
	return Contract{}
}

func exportedInterfaceMethods(t *testing.T, directory string) map[string][]string {
	t.Helper()
	parsed, err := parser.ParseDir(token.NewFileSet(), directory, func(info fs.FileInfo) bool {
		return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string][]string)
	for _, pkg := range parsed {
		for _, file := range pkg.Files {
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, specification := range general.Specs {
					typeSpec, ok := specification.(*ast.TypeSpec)
					if !ok {
						continue
					}
					interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
					if !ok {
						continue
					}
					for _, field := range interfaceType.Methods.List {
						for _, name := range field.Names {
							if name.IsExported() {
								result[typeSpec.Name.Name] = append(result[typeSpec.Name.Name], name.Name)
							}
						}
					}
				}
			}
		}
	}
	return result
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate analyzer test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", ".."))
}

func markdownHasAnchor(content, wanted string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		heading := strings.TrimSpace(line)
		if !strings.HasPrefix(heading, "#") {
			continue
		}
		heading = strings.TrimSpace(strings.TrimLeft(heading, "#"))
		var anchor strings.Builder
		lastHyphen := false
		for _, character := range strings.ToLower(heading) {
			switch {
			case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
				anchor.WriteRune(character)
				lastHyphen = false
			case character == ' ' || character == '-':
				if anchor.Len() > 0 && !lastHyphen {
					anchor.WriteByte('-')
					lastHyphen = true
				}
			}
		}
		if strings.TrimSuffix(anchor.String(), "-") == wanted {
			return true
		}
	}
	return false
}

func exportedSymbols(t *testing.T, directory string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseDir(token.NewFileSet(), directory, func(info fs.FileInfo) bool {
		return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse public package %s: %v", directory, err)
	}
	symbols := make(map[string]bool)
	for _, pkg := range parsed {
		for _, file := range pkg.Files {
			for _, declaration := range file.Decls {
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					if declaration.Recv == nil && declaration.Name.IsExported() {
						symbols[declaration.Name.Name] = true
					} else if declaration.Recv != nil && declaration.Name.IsExported() {
						if receiver := receiverName(declaration.Recv.List[0].Type); receiver != "" {
							symbols[receiver+"."+declaration.Name.Name] = true
						}
					}
				case *ast.GenDecl:
					for _, specification := range declaration.Specs {
						typeSpec, ok := specification.(*ast.TypeSpec)
						if !ok || !typeSpec.Name.IsExported() {
							continue
						}
						symbols[typeSpec.Name.Name] = true
						interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
						if !ok {
							continue
						}
						for _, method := range interfaceType.Methods.List {
							for _, name := range method.Names {
								if name.IsExported() {
									symbols[typeSpec.Name.Name+"."+name.Name] = true
								}
							}
						}
					}
				}
			}
		}
	}
	return symbols
}

func receiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, _ := expression.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
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

func TestContractCorpusCoversInventory(t *testing.T) {
	for _, polarity := range []string{"positive", "negative"} {
		path := filepath.Join("testdata", "contracts", map[string]string{"positive": "compliant", "negative": "noncompliant"}[polarity])
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		var source strings.Builder
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(path, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			source.Write(data)
		}
		for _, contract := range ContractInventory().Contracts {
			annotation := "analyzer-contract: " + contract.ID + " " + polarity
			if !strings.Contains(source.String(), annotation) {
				t.Errorf("%s corpus is missing %q", polarity, contract.ID)
			}
		}
	}
}
