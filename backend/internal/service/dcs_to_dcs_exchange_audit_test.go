package service

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/pgtest"
)

// DCS-NFR-BR-08: every inbound peer exchange that completes leaves an audit
// event; a refused one does not.
func TestReceivedPeerExchangeIsAuditedOnlyWhenItCompletes(t *testing.T) {
	db := pgtest.Open(t)
	svc := &dcsToDcssrvc{DB: db}

	svc.auditReceivedExchange(context.Background(), "post_pdf", "did:web:dcs.example:contract:x", "did:web:peer.example", nil)
	svc.auditReceivedExchange(context.Background(), "post_pdf", "did:web:dcs.example:contract:x", "did:web:intruder.example", errors.New("refused"))

	events := archiveEvents(t, db, "PEER_EXCHANGE")
	require.Len(t, events, 1)
	require.Equal(t, "post_pdf", events[0]["exchange"])
	require.Equal(t, "RECEIVED", events[0]["direction"])
	require.Equal(t, "did:web:peer.example", events[0]["peer_did"])
}

// Each inbound exchange handler records its completion.
func TestEveryInboundPeerExchangeHandlerIsAudited(t *testing.T) {
	for file, handlers := range map[string][]string{
		"dcs_to_dcs.go":            {"PostPdf"},
		"dcs_to_dcs_settlement.go": {"PostSettlement", "PostSettlementWithdrawal"},
	} {
		for _, handler := range handlers {
			if len(callSites(declBody(t, file, handler), "auditReceivedExchange")) != 1 {
				t.Errorf("%s does not audit the exchange it received (DCS-NFR-BR-08)", handler)
			}
		}
	}
}

// declBody returns the body of the named function or method in file.
func declBody(t *testing.T, file, name string) *ast.BlockStmt {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	require.NoError(t, err)
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name && fn.Body != nil {
			return fn.Body
		}
	}
	t.Fatalf("%s declares no %s", file, name)
	return nil
}
