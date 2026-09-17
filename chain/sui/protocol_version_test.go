package sui

import (
	"context"
	"testing"

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

func fetcher(version uint64, calls *int) fetchEpochVersion {
	return func(context.Context, uint64) (uint64, error) {
		*calls++
		return version, nil
	}
}

func TestProtocolGuardAcceptsReviewedVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(137, &calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(1, 1226, 0), fetch))
	// the epoch is resolved once, not once per checkpoint
	require.NoError(t, g.check(context.Background(), checkpointAt(2, 1226, 0), fetch))
	assert.Equal(t, 1, calls)
}

func TestProtocolGuardRejectsNewerEpochVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0

	err := g.check(context.Background(), checkpointAt(42, 1300, 0), fetcher(138, &calls))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol version 138")
	assert.Contains(t, err.Error(), "SENTIO_SUI_MAX_PROTOCOL_VERSION")

	// a rejected epoch is not cached: the next checkpoint in it must fail again
	require.Error(t, g.check(context.Background(), checkpointAt(43, 1300, 0), fetcher(138, &calls)))
}

// An end-of-epoch checkpoint announces the next epoch's version, so the upgrade is caught one
// checkpoint before any transaction can use the new shapes.
func TestProtocolGuardRejectsAnnouncedVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationIOTA, max: 35}
	calls := 0

	err := g.check(context.Background(), checkpointAt(7, 100, 36), fetcher(35, &calls))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is about to switch to protocol version 36")
	assert.Contains(t, err.Error(), "SENTIO_IOTA_MAX_PROTOCOL_VERSION")
	assert.Zero(t, calls, "the announcement is in the checkpoint; no epoch lookup needed to reject")
}

// An accepted end-of-epoch announcement vouches for the next epoch as well (the protocol version
// only moves up), so the guard hands the check from one epoch to the next without ever asking the
// node again.
func TestProtocolGuardAnnouncementCoversBothEpochs(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(137, &calls)

	// last checkpoint of epoch 1226, announcing 137 for 1227
	require.NoError(t, g.check(context.Background(), checkpointAt(100, 1226, 137), fetch))
	assert.Zero(t, calls, "the announcement is authoritative; no epoch lookup needed")

	// both the epoch that announced and the announced one are now covered
	require.NoError(t, g.check(context.Background(), checkpointAt(99, 1226, 0), fetch))
	require.NoError(t, g.check(context.Background(), checkpointAt(101, 1227, 0), fetch))
	assert.Zero(t, calls)
}

// An accepted epoch vouches for every earlier one (the protocol version never moves down), so
// backfilling an older range never asks the node again.
func TestProtocolGuardCoversEarlierEpochs(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	fetch := fetcher(137, &calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(500, 1226, 0), fetch))
	assert.Equal(t, 1, calls)

	require.NoError(t, g.check(context.Background(), checkpointAt(10, 900, 0), fetch))
	require.NoError(t, g.check(context.Background(), checkpointAt(11, 0, 0), fetch))
	assert.Equal(t, 1, calls, "earlier epochs are covered by the accepted one")
}

// Epoch 0 must not pass for free just because nothing has been checked yet.
func TestProtocolGuardChecksEpochZero(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	require.NoError(t, g.check(context.Background(), checkpointAt(0, 0, 0), fetcher(137, &calls)))
	assert.Equal(t, 1, calls)
}

func TestProtocolGuardDisabled(t *testing.T) {
	calls := 0
	// skipValidate leaves the guard nil
	var nilGuard *protocolGuard
	require.NoError(t, nilGuard.check(context.Background(), checkpointAt(1, 1, 999), fetcher(999, &calls)))

	// an override of 0 turns the guard off
	off := &protocolGuard{variation: types.VariationSUI, max: 0}
	require.NoError(t, off.check(context.Background(), checkpointAt(1, 1, 999), fetcher(999, &calls)))
	assert.Zero(t, calls)
}

// A node that does not report a protocol version must not halt the chain.
func TestProtocolGuardToleratesMissingVersion(t *testing.T) {
	g := &protocolGuard{variation: types.VariationSUI, max: 137}
	calls := 0
	require.NoError(t, g.check(context.Background(), checkpointAt(1, 1226, 0), fetcher(0, &calls)))
	assert.Equal(t, 1, calls)
	// nothing was verified, so it is not cached either
	require.NoError(t, g.check(context.Background(), checkpointAt(2, 1226, 0), fetcher(0, &calls)))
	assert.Equal(t, 2, calls)
}
