package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// driverImports are the packages only internal/database may import: the SQLite driver
// itself and database/sql. PostgreSQL's pgx used to be listed here; it went with the
// engine (2026-10-05 业主口径，见根 AGENTS.md §0.6).
func driverImports(name string) bool {
	return name == "modernc.org/sqlite" || name == "database/sql"
}

// Generated persistence types must not become the domain or gateway API. This
// guard also prevents reintroducing a generic SQL transaction facade.
func TestPersistenceBoundary(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleRoot := filepath.Dir(filepath.Dir(wd))
	for _, tree := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(moduleRoot, tree), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(moduleRoot, filepath.Dir(path))
			if err != nil {
				return err
			}
			pkg := filepath.ToSlash(rel)
			isTest := strings.HasSuffix(path, "_test.go")
			isDatabase := pkg == "internal/database" || strings.HasPrefix(pkg, "internal/database/")
			privateImports := map[string]bool{}
			for _, imp := range f.Imports {
				name, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if name == "dfolan/internal/db" {
					t.Errorf("%s imports retired generic database facade", path)
				}
				if name == "dfolan/internal/database/sqlcgen" && !isDatabase {
					t.Errorf("%s exposes generated database types outside database", path)
				}
				if !isDatabase && driverImports(name) {
					t.Errorf("%s imports a database driver outside database", path)
				}
				if driverImports(name) || name == "dfolan/internal/database/sqlcgen" {
					alias := filepath.Base(name)
					if imp.Name != nil {
						alias = imp.Name.Name
					}
					privateImports[alias] = true
				}
			}
			if !isDatabase && !isTest && pkg != "internal/toolcmd/charactercheck" {
				ast.Inspect(f, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if ok && (selector.Sel.Name == "OpenTestFixture" || selector.Sel.Name == "TestFixture") {
						t.Errorf("%s uses an isolated test fixture from production code", path)
					}
					return true
				})
			}
			if !isDatabase && !isTest && pkg != "internal/toolcmd/dbq" {
				ast.Inspect(f, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if ok && selector.Sel.Name == "DiagnosticQuery" {
						t.Errorf("%s uses the dbq diagnostic SQL exception in production", path)
					}
					return true
				})
			}
			if pkg != "internal/database" {
				return nil
			}
			if !isTest {
				for _, decl := range f.Decls {
					method, ok := decl.(*ast.FuncDecl)
					if !ok || !method.Name.IsExported() {
						continue
					}
					// The rule protects the package's public surface. A method on an
					// UNEXPORTED receiver is not part of it: neither the receiver type
					// nor any interface it satisfies can be named from outside the
					// package, so a generated adapter is free to speak in generated
					// types. Exported receivers (Store) are still checked.
					if method.Recv != nil {
						exportedReceiver := false
						if len(method.Recv.List) > 0 {
							switch recvType := method.Recv.List[0].Type.(type) {
							case *ast.StarExpr:
								if ident, ok := recvType.X.(*ast.Ident); ok {
									exportedReceiver = ident.IsExported()
								}
							case *ast.Ident:
								exportedReceiver = recvType.IsExported()
							}
						}
						if !exportedReceiver {
							continue
						}
					}
					ast.Inspect(method.Type, func(node ast.Node) bool {
						selector, ok := node.(*ast.SelectorExpr)
						if ok {
							if qualifier, ok := selector.X.(*ast.Ident); ok && privateImports[qualifier.Name] {
								t.Errorf("%s leaks persistence type through %s", path, method.Name.Name)
							}
						}
						return true
					})
				}
			}
			// Store and Tx are concrete persistence capabilities. Their exported
			// fields cannot hand the driver or generated query API to callers.
			for _, decl := range f.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gen.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok || (typ.Name.Name != "Store" && typ.Name.Name != "Tx") {
						continue
					}
					structure, ok := typ.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, field := range structure.Fields.List {
						if len(field.Names) == 0 {
							t.Errorf("%s embeds a capability in %s", path, typ.Name.Name)
						}
						for _, name := range field.Names {
							if name.IsExported() {
								t.Errorf("%s exposes persistence field %s.%s", path, typ.Name.Name, name.Name)
							}
						}
					}
				}
			}
			for _, decl := range f.Decls {
				method, ok := decl.(*ast.FuncDecl)
				if !ok || method.Recv == nil {
					continue
				}
				receiver := method.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				name, ok := receiver.(*ast.Ident)
				if !ok || (name.Name != "Tx" && name.Name != "Store") {
					continue
				}
				switch method.Name.Name {
				case "Exec", "Query", "QueryRow", "Begin", "BeginTx", "Commit", "Rollback", "Pool", "Driver", "Conn", "Acquire", "Queries":
					t.Errorf("%s exposes generic transaction capability %s", path, method.Name.Name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
