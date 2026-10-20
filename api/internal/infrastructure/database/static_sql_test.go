package database

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every SQL statement in this package is a plain string literal passed to the
// driver together with its parameters. Nothing builds SQL at run time (no
// fmt.Sprintf, no concatenation, no helper that returns SQL), so a value can
// never become part of a statement's text. This test keeps it that way.
func TestSQLIsStaticLiteralsOnly(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// Commands that talk to the database directly follow the same rule.
	cmds, err := filepath.Glob("../../../cmd/*/*.go")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, cmds...)
	// Methods whose second argument is the SQL text: pool/tx Query, QueryRow, Exec
	// and batch Queue.
	sqlArg := map[string]int{"Query": 1, "QueryRow": 1, "Exec": 1, "Queue": 0}
	var violations []string
	fset := token.NewFileSet()
	for _, name := range files {
		// migrations.go applies the .sql files shipped with the application; their
		// text is read from disk and never contains a request's values.
		if strings.HasSuffix(name, "_test.go") || filepath.Base(name) == "migrations.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			// fmt.Sprintf and friends are how SQL gets assembled (in this package;
			// commands may format their own output).
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Sprint") && !strings.Contains(name, "cmd/") {
					violations = append(violations, fset.Position(call.Pos()).String()+": fmt."+sel.Sel.Name)
				}
				if idx, ok := sqlArg[sel.Sel.Name]; ok && len(call.Args) > idx && isDBReceiver(sel.X) {
					if lit, ok := call.Args[idx].(*ast.BasicLit); !ok || lit.Kind != token.STRING {
						violations = append(violations, fset.Position(call.Pos()).String()+": "+sel.Sel.Name+" with a non-literal statement")
					}
				}
			}
			return true
		})
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("SQL must be written out as string literals; found:\n%s", strings.Join(violations, "\n"))
	}
}

// isDBReceiver reports whether x looks like a pool, transaction or batch:
// r.pool, tx, pool, batch.
func isDBReceiver(x ast.Expr) bool {
	switch v := x.(type) {
	case *ast.Ident:
		return v.Name == "tx" || v.Name == "pool" || v.Name == "batch" || v.Name == "db"
	case *ast.SelectorExpr:
		return v.Sel.Name == "pool" || v.Sel.Name == "tx"
	}
	return false
}
