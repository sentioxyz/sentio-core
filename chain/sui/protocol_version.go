package sui

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"

	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/log"
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
// reviewed against. It has two signals:
//
//   - the version an end-of-epoch checkpoint announces for the next epoch. This is the main one:
//     it is consensus data already present in the checkpoint, costs nothing, and fires one
//     checkpoint *before* any transaction can carry the new shapes.
//   - the chain's current version, resolved through GetEpoch once per process. This only covers
//     the gap the announcements cannot: a process that starts up after the upgrade already
//     happened, having never seen the end-of-epoch checkpoint that announced it.
//
// The current version is deliberately not resolved per epoch: nodes keep only a window of recent
// epochs (a lookup a hundred epochs back answers NotFound), so a per-epoch lookup would fail for
// good while backfilling an older range - and a failure here halts the chain.
type protocolGuard struct {
	variation types.Variation
	// max is 0 when the guard is disabled.
	max uint64

	// baselineDone records that the chain's own protocol version has been resolved once and
	// accepted, which is all the startup check is for: from then on every upgrade arrives as an
	// end-of-epoch announcement in the checkpoints themselves. A rejected version never sets it,
	// so every checkpoint keeps failing until the limit is raised.
	baselineDone atomic.Bool
}

func newProtocolGuard(variation types.Variation) *protocolGuard {
	return &protocolGuard{
		variation: variation,
		max:       maxSupportedProtocolVersion(variation),
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

// reject builds the error that halts slot loading, or nil when the version is within range. what
// describes where the version came from, since the two signals mean different things: an
// announcement is about the checkpoint at hand, while the chain's current version may be newer
// than the checkpoint being loaded (backfilling an older range) and still has to halt - the shapes
// we would fail to represent are already on chain.
func (g *protocolGuard) reject(version uint64, what string) error {
	if version <= g.max {
		return nil
	}
	return errors.Errorf(
		"%s %s protocol version %d, above the highest reviewed version %d: this build may not know "+
			"every data shape that version allows, and the grpc path would drop the ones it does not "+
			"know without an error. Sync github.com/sentioxyz/sui-apis with upstream, regenerate, "+
			"review chain/sui/types against the new layout, then raise the limit (override: %s)",
		g.variation, what, version, g.max, g.overrideEnvName())
}

// fetchCurrentVersion resolves the protocol version the chain runs right now.
type fetchCurrentVersion func(ctx context.Context) (uint64, error)

// check validates the version announced by an end-of-epoch checkpoint and, once per process, the
// chain's current version.
func (g *protocolGuard) check(ctx context.Context, ck *rpcv2.Checkpoint, fetch fetchCurrentVersion) error {
	if !g.enabled() {
		return nil
	}
	summary := ck.GetSummary()
	sn, epoch := summary.GetSequenceNumber(), summary.GetEpoch()

	// The announcement is already in the checkpoint: catch the upgrade before the first
	// checkpoint that may carry the new shapes.
	if next := summary.GetEndOfEpochData().GetNextEpochProtocolVersion(); next > 0 {
		if err := g.reject(next, fmt.Sprintf(
			"checkpoint %d ends epoch %d and announces, for epoch %d,", sn, epoch, epoch+1)); err != nil {
			return err
		}
		// The protocol version only ever moves up: a validator votes for next+1 and stops at the
		// highest version a quorum supports (sui-core choose_protocol_version_and_system_
		// packages_v2). So an accepted announcement also proves the chain's version at this
		// checkpoint is within range - exactly what the startup check establishes, from
		// consensus data rather than a node's reply.
		g.baselineDone.Store(true)
		return nil
	}

	if g.baselineDone.Load() {
		return nil
	}

	version, err := fetch(ctx)
	if err != nil {
		return errors.Wrapf(err, "resolve the current protocol version of %s", g.variation)
	}
	if version == 0 {
		// The node did not report one. Stop asking - retrying per checkpoint would put a request
		// per slot on a node that has already shown it will not answer - and rely on the
		// announcements, which come from the checkpoints themselves.
		_, logger := log.FromContext(ctx)
		logger.Warnf("%s node reported no protocol version; the startup check is skipped and an "+
			"unreviewed protocol version will only be caught at the next epoch boundary", g.variation)
		g.baselineDone.Store(true)
		return nil
	}
	if err = g.reject(version, fmt.Sprintf(
		"currently runs, while checkpoint %d (epoch %d) is being loaded,", sn, epoch)); err != nil {
		return err
	}

	g.baselineDone.Store(true)
	return nil
}
