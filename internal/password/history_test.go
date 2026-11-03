package password

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CheckReuse_rejects_candidate_equal_to_current(t *testing.T) {
	// Given
	ctx := context.Background()
	current, err := Hash("old-password-123!")
	require.NoError(t, err)

	// When
	reused, err := CheckReuse(ctx, "old-password-123!", current, nil)

	// Then
	require.NoError(t, err)
	require.True(t, reused)
}

func Test_CheckReuse_rejects_candidate_found_in_history(t *testing.T) {
	// Given
	ctx := context.Background()
	current, err := Hash("current-pass-123!")
	require.NoError(t, err)
	hist := []string{
		"hist-one-456!",
		"hist-two-789!",
		"hist-three-012!",
	}
	hashes := make([]string, len(hist))
	for i, p := range hist {
		h, err := Hash(p)
		require.NoError(t, err)
		hashes[i] = h
	}

	// When
	reused, err := CheckReuse(ctx, "hist-two-789!", current, hashes)

	// Then
	require.NoError(t, err)
	require.True(t, reused)
}

func Test_CheckReuse_allows_candidate_not_in_current_or_history(t *testing.T) {
	// Given
	ctx := context.Background()
	current, err := Hash("current-pass-123!")
	require.NoError(t, err)
	hist1, err := Hash("hist-one-456!")
	require.NoError(t, err)
	hist2, err := Hash("hist-two-789!")
	require.NoError(t, err)

	// When
	reused, err := CheckReuse(ctx, "brand-new-pass-123!", current, []string{hist1, hist2})

	// Then
	require.NoError(t, err)
	require.False(t, reused)
}

func Test_CheckReuse_ignores_empty_current_and_verifies_history_only(t *testing.T) {
	// Given
	ctx := context.Background()
	hist1, err := Hash("old-one-456!")
	require.NoError(t, err)

	// When
	reused, err := CheckReuse(ctx, "old-one-456!", "", []string{hist1})

	// Then
	require.NoError(t, err)
	require.True(t, reused)
}

func Test_CheckReuse_returns_error_on_malformed_hash(t *testing.T) {
	// Given
	ctx := context.Background()

	// When
	_, err := CheckReuse(ctx, "candidate-123!", "not-a-phc-hash", nil)

	// Then
	require.Error(t, err)
}

func Test_CheckReuse_aborts_when_context_cancelled(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	current, err := Hash("current-pass-123!")
	require.NoError(t, err)

	// When
	_, err = CheckReuse(ctx, "candidate-123!", current, nil)

	// Then
	require.ErrorIs(t, err, context.Canceled)
}
