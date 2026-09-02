package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// localFunctionBindings retains only declaration-initialized local variables
// with one write and no address escape anywhere in the package's syntax.
// Object identity keeps shadowed names and captured writes distinct.
func localFunctionBindings(pass *analysis.Pass) map[*types.Var]ast.Expr {
	bindings := make(map[*types.Var]ast.Expr)
	writes := make(map[*types.Var]int)
	addressed := make(map[*types.Var]bool)
	variable := func(expression ast.Expr) *types.Var {
		identifier, ok := ast.Unparen(expression).(*ast.Ident)
		if !ok {
			return nil
		}
		object, _ := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
		return object
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.ValueSpec:
				for index, name := range node.Names {
					object, _ := pass.TypesInfo.Defs[name].(*types.Var)
					writes[object]++
					if len(node.Names) == len(node.Values) {
						bindings[object] = node.Values[index]
					}
				}
			case *ast.AssignStmt:
				for index, lhs := range node.Lhs {
					object := variable(lhs)
					writes[object]++
					if identifier, ok := lhs.(*ast.Ident); ok && node.Tok == token.DEFINE && pass.TypesInfo.Defs[identifier] == object && len(node.Lhs) == len(node.Rhs) {
						bindings[object] = node.Rhs[index]
					}
				}
			case *ast.RangeStmt:
				if node.Key != nil {
					writes[variable(node.Key)]++
				}
				if node.Value != nil {
					writes[variable(node.Value)]++
				}
			case *ast.UnaryExpr:
				if node.Op == token.AND {
					addressed[variable(node.X)] = true
				}
			}
			return true
		})
	}
	for object := range bindings {
		if object == nil || object.Parent() == pass.Pkg.Scope() || writes[object] != 1 || addressed[object] {
			delete(bindings, object)
			continue
		}
		if _, function := object.Type().Underlying().(*types.Signature); !function {
			delete(bindings, object)
		}
	}
	return bindings
}

func resolvedDeviceCallee(pass *analysis.Pass, call *ast.CallExpr, bindings map[*types.Var]ast.Expr) *types.Func {
	value := resolvedDeviceFunctionValue(pass, call.Fun, bindings)
	if value == nil {
		return nil
	}
	return typeutil.StaticCallee(pass.TypesInfo, &ast.CallExpr{Fun: value})
}

func resolvedDeviceFunctionValue(pass *analysis.Pass, expression ast.Expr, bindings map[*types.Var]ast.Expr) ast.Expr {
	seen := make(map[*types.Var]bool)
	for {
		expression = ast.Unparen(expression)
		identifier, ok := expression.(*ast.Ident)
		if !ok {
			return expression
		}
		object, _ := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
		value := bindings[object]
		if value == nil {
			return expression
		}
		if seen[object] {
			return nil
		}
		seen[object] = true
		expression = value
	}
}
