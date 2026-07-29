package service

import (
	"context"
	"strings"
	"testing"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
)

func TestVerifyInboundPoAEvidenceMissingFailsClosed(t *testing.T) {
	s := &dcsToDcssrvc{}
	_, _, err := s.verifyInboundPoAEvidence(context.Background(), &dcstodcs.DCSToDCSContractPdfRequest{
		ContractIri: "did:example:contract",
	})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing PoA evidence error = %v", err)
	}
}
