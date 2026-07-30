package command

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/lib/pq"
)

// A lock wait cut short by lock_timeout means the background regenerator still
// holds the contract. Reporting it as ErrRegenerationInFlight is what keeps the
// caller from reading a generic database failure into contention that resolves
// on its own — the condition the whole bounded wait exists to surface.
func TestRegenerationLockErrorNamesContention(t *testing.T) {
	err := regenerationLockError("did:contract:1", &pq.Error{Code: pqLockNotAvailable, Message: "canceling statement due to lock timeout"})

	if !errors.Is(err, ErrRegenerationInFlight) {
		t.Fatalf("a lock_timeout must report ErrRegenerationInFlight, got %v", err)
	}
	if !strings.Contains(err.Error(), "did:contract:1") {
		t.Fatalf("the contract must be named, got %v", err)
	}
}

// Any other failure of the lock statement is a database failure, not
// contention: reporting it as retry-later would tell the caller to wait out a
// condition that is not going to clear.
func TestRegenerationLockErrorKeepsOtherFailuresDistinct(t *testing.T) {
	for name, cause := range map[string]error{
		"other postgres error": &pq.Error{Code: "42P01", Message: "relation does not exist"},
		"transport failure":    errors.New("connection reset by peer"),
	} {
		t.Run(name, func(t *testing.T) {
			err := regenerationLockError("did:contract:1", cause)

			if errors.Is(err, ErrRegenerationInFlight) {
				t.Fatalf("%v must not be reported as regeneration contention", cause)
			}
			if !errors.Is(err, cause) {
				t.Fatalf("the cause must be preserved, got %v", err)
			}
		})
	}
}

// The regenerator decides whether to leave a contract alone under the
// per-contract advisory lock, and its answer depends on the signature the
// signing path writes. That argument only holds if the WRITER holds the lock
// too: otherwise the sweep picks up an APPROVED contract with no stored CID,
// the submit spends seconds in DSS validation, the regenerator takes the free
// lock and fresh-renders, and whichever UPDATE commits last owns pdf_ipfs_cid.
// The invariant is per transaction, not per function — finalize writes the row
// inside its caller's transaction — so this walks the package's call graph:
// every transaction that reaches a signature write must reach the lock.
func TestEveryTransactionThatRecordsASignatureTakesTheRegenerationLock(t *testing.T) {
	graph := packageCallGraph(t)

	for name, calls := range graph {
		if !calls["BeginTxx"] {
			continue
		}
		if !reaches(graph, name, "CreateSignature") && !reaches(graph, name, "SetSignedPDF") {
			continue
		}
		if !reaches(graph, name, "acquireRegenerationLock") {
			t.Errorf("%s opens the transaction that records a signature but never takes the per-contract regeneration lock", name)
		}
	}
}

// packageCallGraph maps every function declared in this package to the names it
// calls, closures included.
func packageCallGraph(t *testing.T) map[string]map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	fset := token.NewFileSet()
	graph := map[string]map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			calls := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					calls[fun.Name] = true
				case *ast.SelectorExpr:
					calls[fun.Sel.Name] = true
				}
				return true
			})
			graph[fn.Name.Name] = calls
		}
	}
	if len(graph) == 0 {
		t.Fatal("the package parsed to no functions")
	}
	return graph
}

// reaches reports whether from calls target directly or through any chain of
// functions declared in this package.
func reaches(graph map[string]map[string]bool, from, target string) bool {
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(name string) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		for callee := range graph[name] {
			if callee == target || walk(callee) {
				return true
			}
		}
		return false
	}
	return walk(from)
}
