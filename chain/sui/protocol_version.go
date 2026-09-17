package sui

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"

	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/envconf"
)

// Sui and IOTA gate every new on-chain data shape behind a protocol version: a new
// TransactionExpiration variant, a new FundsWithdrawal source or a new end-of-epoch kind only
// becomes possible once the chain reaches the protocol version that enables it. Our decoders are
// written against one snapshot of those shapes (the protos vendored in
// github.com/sentioxyz/sui-apis and the BCS types in chain/sui/types), so a protocol version
// nobody has reviewed may carry shapes this build cannot represent.
//
// On the json-rpc path that is safe by construction: TxSanityCheck requires the decoded
// transaction to re-encode byte-for-byte, so an unknown shape fails loudly and halts slot loading.
// The grpc path has no such backstop - proto3 drops unknown enum values and unknown fields without
// an error, so an unreviewed protocol version is persisted with fields silently missing. This
// guard gives the grpc path the same halt-instead-of-guess behavior.
//
// Raising these numbers is a deliberate review step, not a version bump: sync
// github.com/sentioxyz/sui-apis with upstream, regenerate the bindings, diff the new shapes
// against chain/sui/types (the sui-types staged snapshot is the authoritative BCS layout), and
// only then raise the constant here. To unblock a chain before that work lands - having checked
// that the new version changes nothing we read - raise the matching environment variable instead.
const (
	// maxSupportedSuiProtocolVersion is the highest Sui protocol version whose data shapes have
	// been reviewed against this build. 137 added TransactionExpiration::Validity (with
	// allowed_proposers) and the FundsWithdrawal SENDER_ALLOWANCE source.
	maxSupportedSuiProtocolVersion = 137
	// maxSupportedIotaProtocolVersion is the same for IOTA, whose protocol versions are numbered
	// independently of Sui's.
	maxSupportedIotaProtocolVersion = 35
)

// Overrides for the constants above; 0 disables the guard for that variation entirely.
var (
	configuredMaxSuiProtocolVersion = envconf.LoadUInt64(
		"SENTIO_SUI_MAX_PROTOCOL_VERSION", maxSupportedSuiProtocolVersion)
	configuredMaxIotaProtocolVersion = envconf.LoadUInt64(
		"SENTIO_IOTA_MAX_PROTOCOL_VERSION", maxSupportedIotaProtocolVersion)
)

func maxSupportedProtocolVersion(variation types.Variation) uint64 {
	if variation == types.VariationIOTA {
		return configuredMaxIotaProtocolVersion
	}
	return configuredMaxSuiProtocolVersion
}

// protocolGuard halts slot loading when the chain runs a protocol version this build has not been
// reviewed against. It checks two things per checkpoint, both cheap:
//
//   - the epoch the checkpoint belongs to, resolved once per epoch through the node (one call per
//     epoch, i.e. roughly one per day);
//   - the next epoch's version announced in an end-of-epoch checkpoint, which costs nothing and
//     catches an upgrade one checkpoint *before* the new shapes can appear.
type protocolGuard struct {
	variation types.Variation
	// max is 0 when the guard is disabled.
	max uint64

	mu sync.Mutex
	// checkedEpochs holds the epochs whose protocol version was resolved and accepted, so the
	// node is asked once per epoch rather than once per checkpoint. Rejected epochs are not
	// recorded: every checkpoint in them must keep failing.
	checkedEpochs map[uint64]bool
}

func newProtocolGuard(variation types.Variation) *protocolGuard {
	return &protocolGuard{
		variation:     variation,
		max:           maxSupportedProtocolVersion(variation),
		checkedEpochs: map[uint64]bool{},
	}
}

func (g *protocolGuard) enabled() bool {
	return g != nil && g.max > 0
}

func (g *protocolGuard) overrideEnvName() string {
	if g.variation == types.VariationIOTA {
		return "SENTIO_IOTA_MAX_PROTOCOL_VERSION"
	}
	return "SENTIO_SUI_MAX_PROTOCOL_VERSION"
}

func (g *protocolGuard) accept(version uint64, epoch uint64, sn uint64, announced bool) error {
	if version <= g.max {
		return nil
	}
	what := "runs"
	if announced {
		what = "is about to switch to"
	}
	return errors.Errorf(
		"%s checkpoint %d (epoch %d) %s protocol version %d, above the highest reviewed version %d: "+
			"this build may not know every data shape that version allows, and the grpc path would drop "+
			"the ones it does not know without an error. Sync github.com/sentioxyz/sui-apis with upstream, "+
			"regenerate, review chain/sui/types against the new layout, then raise the limit (override: %s)",
		g.variation, sn, epoch, what, version, g.max, g.overrideEnvName())
}

// fetchEpochVersion resolves the protocol version of one epoch.
type fetchEpochVersion func(ctx context.Context, epoch uint64) (uint64, error)

// check validates the checkpoint's own epoch and, when the checkpoint ends an epoch, the version
// announced for the next one. fetch is called at most once per epoch.
func (g *protocolGuard) check(ctx context.Context, ck *rpcv2.Checkpoint, fetch fetchEpochVersion) error {
	if !g.enabled() {
		return nil
	}
	summary := ck.GetSummary()
	sn, epoch := summary.GetSequenceNumber(), summary.GetEpoch()

	// The end-of-epoch announcement is already in the checkpoint: catch the upgrade before the
	// first checkpoint that may carry the new shapes.
	if next := summary.GetEndOfEpochData().GetNextEpochProtocolVersion(); next > 0 {
		if err := g.accept(next, epoch+1, sn, true); err != nil {
			return err
		}
		// The protocol version only ever moves up: a validator votes for next+1 and stops at
		// the highest version a quorum supports (sui-core choose_protocol_version_and_system_
		// packages_v2), so an accepted next epoch vouches for this one too. The announcement is
		// consensus data rather than a node's reply, so trust it for both epochs and skip the
		// lookup — while slots are loaded in order, this hands the check from one epoch to the
		// next and GetEpoch is never called again.
		g.markChecked(epoch, epoch+1)
		return nil
	}

	if g.isChecked(epoch) {
		return nil
	}

	version, err := fetch(ctx, epoch)
	if err != nil {
		return errors.Wrapf(err, "resolve protocol version of epoch %d", epoch)
	}
	if version == 0 {
		// The node did not report one; nothing to check against, and failing here would halt the
		// chain on an unrelated gap in the reply.
		return nil
	}
	if err = g.accept(version, epoch, sn, false); err != nil {
		return err
	}

	g.markChecked(epoch)
	return nil
}

func (g *protocolGuard) isChecked(epoch uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.checkedEpochs[epoch]
}

func (g *protocolGuard) markChecked(epochs ...uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, epoch := range epochs {
		g.checkedEpochs[epoch] = true
	}
}
