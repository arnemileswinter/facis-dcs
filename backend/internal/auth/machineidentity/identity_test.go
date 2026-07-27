package machineidentity

import "testing"

func TestRolesRoundTrip(t *testing.T) {
	encoded, err := EncodeRoles([]string{"Sys. Contract Creator", "Sys. Auditor"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	roles, err := Identity{RolesJSON: encoded}.Roles()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(roles) != 2 || roles[0] != "Sys. Contract Creator" || roles[1] != "Sys. Auditor" {
		t.Fatalf("roles did not survive the round trip: %v", roles)
	}
}

// An identity with no roles could authenticate and then do nothing, with
// nothing to say why, so it is refused when it is written rather than when it
// first calls.
func TestEncodeRolesRefusesAnEmptyList(t *testing.T) {
	if _, err := EncodeRoles(nil); err == nil {
		t.Fatal("an identity with no roles was accepted")
	}
}

func TestRolesReportsUnreadableStorage(t *testing.T) {
	if _, err := (Identity{Name: "broken", RolesJSON: "not json"}).Roles(); err == nil {
		t.Fatal("an unreadable role list was accepted")
	}
}

// A row written before any roles existed decodes as none rather than failing,
// so the caller is refused by having no authority instead of by an error.
func TestEmptyRolesJSONDecodesAsNone(t *testing.T) {
	roles, err := (Identity{RolesJSON: "  "}).Roles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected no roles, got %v", roles)
	}
}
