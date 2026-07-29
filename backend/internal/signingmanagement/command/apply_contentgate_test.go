package command

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubMatcher records what SubmitSignature's content gate compares and returns a
// canned verdict.
type stubMatcher struct {
	submitted []byte
	reference []byte
	match     bool
	mismatch  string
	err       error
}

func (s *stubMatcher) MatchContent(_ context.Context, submitted, reference []byte) (bool, string, error) {
	s.submitted = submitted
	s.reference = reference
	return s.match, s.mismatch, s.err
}

// The gate compares the submission against the PINNED prepared bytes — not a
// fresh render — so acceptance never depends on reproducing a render.
func TestAssertPreparedContentComparesAgainstThePinnedDocument(t *testing.T) {
	m := &stubMatcher{match: true}
	submitted := []byte("%PDF submitted")
	prepared := []byte("%PDF pinned at prepare")

	require.NoError(t, assertPreparedContent(context.Background(), m, submitted, prepared, "Signature1"))
	require.Equal(t, submitted, m.submitted)
	require.Equal(t, prepared, m.reference)
}

// A submission whose visible pages no longer render the prepared document is
// REFUSED with a typed error naming the field and what diverged — a caller
// mapping this to a client rejection must not have to string-match a 500.
func TestAssertPreparedContentRefusesADivergentSubmission(t *testing.T) {
	m := &stubMatcher{
		match:    false,
		mismatch: `page 1 content does not match compiled output (at byte 412: candidate="(Substituted clause"...)`,
	}

	err := assertPreparedContent(context.Background(), m, []byte("%PDF tampered"), []byte("%PDF pinned"), "Signature1")
	require.ErrorIs(t, err, ErrContentMismatch)
	require.Contains(t, err.Error(), "Signature1")
	require.Contains(t, err.Error(), "page 1 content does not match")
	require.Contains(t, err.Error(), "Substituted clause")
}

// An unreachable or failing comparison refuses the submission too: a signature
// is accepted only on a positive match, never on the absence of a negative.
func TestAssertPreparedContentRefusesWhenTheComparisonFails(t *testing.T) {
	m := &stubMatcher{err: errors.New("pdf-core unreachable")}

	err := assertPreparedContent(context.Background(), m, []byte("a"), []byte("b"), "Signature1")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrContentMismatch)
	require.Contains(t, err.Error(), "pdf-core unreachable")
}
