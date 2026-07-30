package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// "A field can never be signed twice" (DCS-FR-SM-07/-17) is a rule two
// concurrent submits can only be held to by the database: each reads the other's
// row before it exists, so whichever check the application runs, the constraint
// is what makes a second SIGNED row impossible. 20260714c stated the rule and
// created a plain index.
//
// It must be PARTIAL: a revoked signature keeps its row and the field may be
// signed again, and signatures predating field_name carry none — a unique index
// over the bare column pair would reject both.
func TestOneSignedSignaturePerFieldIsEnforcedByAUniqueIndex(t *testing.T) {
	statement := regexp.MustCompile(
		`(?is)CREATE\s+UNIQUE\s+INDEX[^;]*?ON\s+contract_signatures\s*\(\s*contract_did\s*,\s*field_name\s*\)([^;]*);`)

	found := ""
	for name, sql := range migrationSQL(t) {
		if m := statement.FindStringSubmatch(sql); m != nil {
			found = name + ": " + m[1]
			break
		}
	}
	if found == "" {
		t.Fatal("no unique index on contract_signatures (contract_did, field_name): a second SIGNED row for one field is only prevented by application code")
	}
	if !strings.Contains(found, "WHERE") || !strings.Contains(found, "'SIGNED'") {
		t.Errorf("the unique index is not restricted to SIGNED rows, so a revoked-then-re-signed field would be rejected: %s", found)
	}
	if !strings.Contains(found, "field_name IS NOT NULL") {
		t.Errorf("the unique index is not restricted to rows that name a field, so the single-signer rows predating field_name would collide: %s", found)
	}
}

// migrationSQL returns the contents of every embedded migration file.
func migrationSQL(t *testing.T) map[string]string {
	t.Helper()
	entries, err := fs.ReadDir(sqlFiles, "sql")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	contents := make(map[string]string, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := fs.ReadFile(sqlFiles, "sql/"+entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		contents[entry.Name()] = string(body)
	}
	return contents
}
