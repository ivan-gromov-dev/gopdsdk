package analyzer

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/scanner"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
)

func devicePanicCleanupRule(providers Providers) *analysis.Analyzer {
	return &analysis.Analyzer{Name: "devicepaniccleanup", Doc: "find explicit panics with potentially pending deferred calls", Requires: []*analysis.Analyzer{providers.ControlFlow}, Run: func(pass *analysis.Pass) (any, error) {
		graphs := pass.ResultOf[providers.ControlFlow].(*ctrlflow.CFGs)
		for _, file := range pass.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				var graph *cfg.CFG
				switch function := node.(type) {
				case *ast.FuncDecl:
					if function.Body != nil {
						graph = graphs.FuncDecl(function)
					}
				case *ast.FuncLit:
					graph = graphs.FuncLit(function)
				}
				if graph != nil {
					reportPendingDeferPanic(pass, graph)
				}
				return true
			})
		}
		return nil, nil
	}}
}

func reportPendingDeferPanic(pass *analysis.Pass, graph *cfg.CFG) {
	type state struct {
		block   *cfg.Block
		pending bool
	}
	queue := []state{{block: graph.Blocks[0]}}
	seen := make(map[state]bool)
	reported := make(map[token.Pos]bool)
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true
		for _, node := range current.block.Nodes {
			if _, ok := node.(*ast.DeferStmt); ok {
				current.pending = true
			}
			statement, ok := node.(*ast.ExprStmt)
			if !ok || !current.pending {
				continue
			}
			call, ok := statement.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			identifier, ok := ast.Unparen(call.Fun).(*ast.Ident)
			if !ok {
				continue
			}
			builtin, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Builtin)
			if ok && builtin.Name() == "panic" && !reported[call.Pos()] {
				reported[call.Pos()] = true
				pass.Reportf(call.Pos(), "device panic traps without running deferred calls that may be pending on this path")
			}
		}
		for _, successor := range current.block.Succs {
			queue = append(queue, state{successor, current.pending})
		}
	}
}

func reportDeviceLinkname(pass *analysis.Pass, node ast.Node) {
	file, ok := node.(*ast.File)
	if !ok {
		return
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			fields := strings.Fields(comment.Text)
			if len(fields) != 3 || fields[0] != "//go:linkname" {
				continue
			}
			target := fields[2]
			index := strings.LastIndex(target, ".")
			if index < 1 {
				continue
			}
			path, name := target[:index], target[index+1:]
			// Only public package functions are classified here. Method linker
			// names and private ABI entry points require separate contracts.
			if !ast.IsExported(name) {
				continue
			}
			pkg := types.NewPackage(path, filepath.Base(path))
			function := types.NewFunc(token.NoPos, pkg, name, types.NewSignatureType(nil, nil, nil, nil, nil, false))
			if rule, forbidden := forbiddenPackageRule(function); forbidden {
				pass.Reportf(comment.Pos(), "go:linkname target %s is unavailable on device (%s)", target, rule)
			}
		}
	}
}

func deviceAssemblyRule() *analysis.Analyzer {
	return &analysis.Analyzer{Name: "devicegoassembly", Doc: "find Go declarations implemented in selected Go assembly", Run: func(pass *analysis.Pass) (any, error) {
		comments := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`)
		symbols := regexp.MustCompile(`(?m)^\s*TEXT\s+·([\pL_][\pL\pN_]*)\(SB\)`)
		implementations := make(map[string]string)
		for _, name := range pass.OtherFiles {
			if filepath.Ext(name) != ".s" {
				continue
			}
			data, err := pass.ReadFile(name)
			if err != nil {
				return nil, fmt.Errorf("read assembly %s: %w", name, err)
			}
			if deviceSourceExcluded(name, data) {
				continue
			}
			for _, match := range symbols.FindAllStringSubmatch(comments.ReplaceAllString(string(data), ""), -1) {
				base := filepath.Base(name)
				if old := implementations[match[1]]; old == "" || base < old {
					implementations[match[1]] = base
				}
			}
		}
		for _, file := range pass.Files {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Body != nil || function.Recv != nil {
					continue
				}
				if name := implementations[function.Name.Name]; name != "" {
					pass.Reportf(function.Name.Pos(), "Go assembly implementation in %s is not supported by TinyGo; provide a Go implementation for device", name)
				}
			}
		}
		return nil, nil
	}}
}

func deviceBuildConstraintRule() *analysis.Analyzer {
	return &analysis.Analyzer{Name: "devicebuildconstraint", Doc: "identify host-selected files excluded by fixed device constraints", Run: func(pass *analysis.Pass) (any, error) {
		for _, file := range pass.Files {
			name := pass.Fset.Position(file.Pos()).Filename
			data, err := pass.ReadFile(name)
			if err != nil {
				return nil, err
			}
			if deviceSourceExcluded(name, data) {
				pass.Reportf(file.Name.Pos(), "host-selected source is excluded by device build constraints; verify the device-specific implementation")
			}
		}
		return nil, nil
	}}
}

// Cortex-M7 inherits linux/arm, baremetal and cortexm from the installed
// TinyGo target; gopdsdk adds qemu. Unknown application/version tags remain
// unconstrained so they cannot alone establish exclusion.
func deviceFixedTag(tag string) (bool, bool) {
	switch tag {
	case "cgo":
		// Application cgo is forbidden by its own rule, but that does not
		// determine the compiler's cgo build-tag setting.
		return false, false
	case "linux", "arm", "unix", "tinygo", "baremetal", "cortexm", "qemu":
		return true, true
	}
	return false, hostBuildTag(tag)
}

func deviceSourceExcluded(name string, data []byte) bool {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	base = strings.TrimSuffix(base, "_test")
	parts := strings.Split(base, "_")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if filenameDeviceExcluded(last) {
			return true
		}
		if last == "arm" && len(parts) > 2 && filenameOSExcluded(parts[len(parts)-2]) {
			return true
		}
	}
	expression := deviceSourceConstraint(data)
	if expression == nil {
		return false
	}
	var unknown []string
	for tag := range constraintTags(expression) {
		if _, known := deviceFixedTag(tag); !known {
			unknown = append(unknown, tag)
		}
	}
	if len(unknown) > 12 {
		return false
	}
	for mask := 0; mask < 1<<len(unknown); mask++ {
		if expression.Eval(func(tag string) bool {
			if value, known := deviceFixedTag(tag); known {
				return value
			}
			for index, name := range unknown {
				if name == tag {
					return mask&(1<<index) != 0
				}
			}
			return false
		}) {
			return false
		}
	}
	return true
}

func deviceSourceConstraint(data []byte) constraint.Expr {
	file := token.NewFileSet().AddFile("source", -1, len(data))
	var lexer scanner.Scanner
	lexer.Init(file, data, func(token.Position, string) {}, scanner.ScanComments)
	var legacy constraint.Expr
	for {
		_, kind, text := lexer.Scan()
		if kind != token.COMMENT {
			return legacy
		}
		if constraint.IsGoBuild(text) {
			expression, _ := constraint.Parse(text)
			return expression
		}
		if constraint.IsPlusBuild(text) {
			expression, err := constraint.Parse(text)
			if err != nil {
				return nil
			}
			if legacy == nil {
				legacy = expression
			} else {
				legacy = &constraint.AndExpr{X: legacy, Y: expression}
			}
		}
	}
}

func filenameOSExcluded(tag string) bool {
	switch tag {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "js", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows":
		return true
	}
	return false
}

func filenameDeviceExcluded(tag string) bool {
	if filenameOSExcluded(tag) {
		return true
	}
	switch tag {
	case "386", "amd64", "amd64p32", "arm64", "loong64", "mips", "mips64", "mips64le", "mipsle", "ppc64", "ppc64le", "riscv64", "s390x", "sparc64", "wasm":
		return true
	}
	return false
}
