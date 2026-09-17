package sui

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"

	"sentioxyz/sentio-core/chain/sui/types"
)

func checkpointAt(sn, epoch uint64, nextEpochVersion uint64) *rpcv2.Checkpoint {
	summary := &rpcv2.CheckpointSummary{SequenceNumber: &sn, Epoch: &epoch}
	if nextEpochVersion > 0 {
		summary.EndOfEpochData = &rpcv2.EndOfEpochData{NextEpochProtocolVersion: &nextEpochVersion}
	}
	return &rpcv2.Checkpoint{Summary: summary}
}

func fetcher(version uint64, calls *int) fetchCurrentVersion {
	return func(context.Context) (uint64, error) {
		*calls++
		return version, nil
	}
}

func TestProtocolGuardAcceptsReviewedVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(137, &calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(1, 1225, 0), fetch))
	// the current version is resolved once per process, not once per checkpoint
	require.NoError(t, g.check(context.Background(), checkpointAt(2, 1225, 0), fetch))
	require.NoError(t, g.check(context.Background(), checkpointAt(3, 1226, 0), fetch))
	assert.Equal(t, 1, calls)
}

func TestProtocolGuardRejectsNewerCurrentVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0

	err := g.check(context.Background(), checkpointAt(42, 1300, 0), fetcher(138, &calls))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "currently runs, while checkpoint 42 (epoch 1300) is being loaded")
	assert.Contains(t, err.Error(), "protocol version 138")
	assert.Contains(t, err.Error(), "SENTIO_SUI_MAX_PROTOCOL_VERSION")

	// a rejected version is never cached: the next checkpoint must fail again
	require.Error(t, g.check(context.Background(), checkpointAt(43, 1300, 0), fetcher(138, &calls)))
	assert.Equal(t, 2, calls)
}

// An end-of-epoch checkpoint announces the next epoch's version, so an upgrade is caught one
// checkpoint before any transaction can use the new shapes.
func TestProtocolGuardRejectsAnnouncedVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationIOTA, max: 35}
	calls := 0

	err := g.check(context.Background(), checkpointAt(7, 100, 36), fetcher(35, &calls))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "announces, for epoch 101, protocol version 36")
	assert.Contains(t, err.Error(), "SENTIO_IOTA_MAX_PROTOCOL_VERSION")
	assert.Zero(t, calls, "the announcement is in the checkpoint; no lookup needed to reject")
}

// An accepted announcement proves the chain is within range too (the version never moves down),
// so it also satisfies the startup check - without asking the node at all.
func TestProtocolGuardAnnouncementSatisfiesBaseline(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(137, &calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(100, 1226, 137), fetch))
	require.NoError(t, g.check(context.Background(), checkpointAt(101, 1227, 0), fetch))
	assert.Zero(t, calls)
}

// A later announcement is still checked after the baseline is satisfied - that is the signal that
// catches every upgrade after startup.
func TestProtocolGuardChecksAnnouncementAfterBaseline(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(137, &calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(1, 1225, 0), fetch))
	require.Error(t, g.check(context.Background(), checkpointAt(500, 1225, 138), fetch))
}

// Resolving the current version is a per-process startup check, so a failure must halt the chain
// rather than pass unchecked - slot loading retries, so a transient failure heals itself.
func TestProtocolGuardFailedLookupHalts(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	fetch := func(context.Context) (uint64, error) { return 0, errors.New("upstream is down") }

	err := g.check(context.Background(), checkpointAt(1, 1225, 0), fetch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream is down")
}

// A node that reports no version must not halt the chain, and must not be asked once per
// checkpoint either.
func TestProtocolGuardToleratesMissingVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(0, &calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(1, 1225, 0), fetch))
	require.NoError(t, g.check(context.Background(), checkpointAt(2, 1225, 0), fetch))
	assert.Equal(t, 1, calls)

	// announcements still apply
	require.Error(t, g.check(context.Background(), checkpointAt(3, 1225, 138), fetch))
}

func TestProtocolGuardDisabled(t *testing.T) {
	calls := 0
	// skip-validate leaves the guard nil
	var nilGuard *protocolGuard
	require.NoError(t, nilGuard.check(context.Background(), checkpointAt(1, 1, 999), fetcher(999, &calls)))

	// an override of 0 turns the guard off
	off := &protocolGuard{variation: types.VariationSUI, max: 0}
	require.NoError(t, off.check(context.Background(), checkpointAt(1, 1, 999), fetcher(999, &calls)))
	assert.Zero(t, calls)
}
