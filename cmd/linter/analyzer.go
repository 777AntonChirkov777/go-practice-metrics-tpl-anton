package main

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var analyzer = &analysis.Analyzer{
	Name: "panicexit",
	Doc: "report uses of builtin panic, and of os.Exit and log.Fatal outside func main of package main\n\n" +
		"Every use of the builtin panic is reported. Uses of os.Exit, log.Fatal, log.Fatalf, log.Fatalln " +
		"and of the Fatal, Fatalf and Fatalln methods of *log.Logger are reported unless they appear " +
		"in the body of func main of package main. Test files and generated files are not checked.",
	Run: run,
}

var builtinPanic = types.Universe.Lookup("panic")

var processExits = map[string]bool{
	"os.Exit":               true,
	"log.Fatal":             true,
	"log.Fatalf":            true,
	"log.Fatalln":           true,
	"(*log.Logger).Fatal":   true,
	"(*log.Logger).Fatalf":  true,
	"(*log.Logger).Fatalln": true,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if isTestFile(pass, file) || ast.IsGenerated(file) {
			continue
		}
		for _, decl := range file.Decls {
			checkDecl(pass, decl, isMainFunc(pass, decl))
		}
	}
	return nil, nil
}

func checkDecl(pass *analysis.Pass, decl ast.Decl, insideMain bool) {
	var visit func(ast.Node) bool
	visit = func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			name, ok := processExitName(pass, n.Sel)
			if !ok {
				return true
			}
			if !insideMain {
				reportProcessExit(pass, n, name)
			}
			ast.Inspect(n.X, visit)
			return false
		case *ast.Ident:
			if pass.TypesInfo.Uses[n] == builtinPanic {
				pass.ReportRangef(n, "use of builtin panic")
				return true
			}
			if name, ok := processExitName(pass, n); ok && !insideMain {
				reportProcessExit(pass, n, name)
			}
		}
		return true
	}
	ast.Inspect(decl, visit)
}

func processExitName(pass *analysis.Pass, id *ast.Ident) (string, bool) {
	fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
	if !ok {
		return "", false
	}
	name := fn.FullName()
	return name, processExits[name]
}

func reportProcessExit(pass *analysis.Pass, rng analysis.Range, name string) {
	pass.ReportRangef(rng, "use of %s outside function main of package main", name)
}

func isMainFunc(pass *analysis.Pass, decl ast.Decl) bool {
	fn, ok := decl.(*ast.FuncDecl)
	return ok && fn.Recv == nil && fn.Name.Name == "main" && pass.Pkg.Name() == "main"
}

func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go")
}
