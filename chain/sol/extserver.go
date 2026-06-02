package sol

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	rg "sentioxyz/sentio-core/common/range"
)

// ExtServerDimension loads Solana slots (with full parsed transactions) directly from a node, used
// both as the source dimension of the sync-chain task and as the latest-slot cache's node loader.
type ExtServerDimension struct {
	client *ClientPool

	*chain.ExtServerDimension[*Slot]
}

var _ chain.Dimension[*Slot] = (*ExtServerDimension)(nil)

func NewExtServerDimension(
	client *ClientPool,
	loadConcurrency uint,
	loadBatchSize uint,
	loadRetry int,
	validRange rg.Range,
	fallBehind time.Duration,
) *ExtServerDimension {
	dim := &ExtServerDimension{client: client}
	dim.ExtServerDimension = chain.NewExtServerDimension[*Slot](
		client,
		loadConcurrency,
		loadBatchSize,
		loadRetry,
		validRange,
		fallBehind,
		dim)
	return dim
}

func (d *ExtServerDimension) GetSlotHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	return d.getSlot(ctx, "ext.GetSlotHeader", sn)
}

func (d *ExtServerDimension) GetSlots(ctx context.Context, sr rg.Range) ([]*Slot, error) {
	slots := make([]*Slot, 0, *sr.Size())
	for sn := sr.Start; sn <= *sr.End; sn++ {
		slot, err := d.getSlot(ctx, "ext.GetSlots", sn)
		if err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	return slots, nil
}

var slotSkippedErrorMatcher = regexp.MustCompile(`slot.*was skipped`)

func isSlotSkippedError(err error) bool {
	return err != nil && slotSkippedErrorMatcher.FindString(strings.ToLower(err.Error())) != ""
}

// rawParsedBlock is the subset of the getBlock (jsonParsed/full) response we persist.
type rawParsedBlock struct {
	Blockhash         solana.Hash                 `json:"blockhash"`
	PreviousBlockhash solana.Hash                 `json:"previousBlockhash"`
	ParentSlot        uint64                      `json:"parentSlot"`
	BlockHeight       *uint64                     `json:"blockHeight"`
	BlockTime         *solana.UnixTimeSeconds     `json:"blockTime"`
	Transactions      []ParsedTransactionWithMeta `json:"transactions"`
}

var getParsedBlockOpts = rpc.M{
	"encoding":                       solana.EncodingJSONParsed,
	"transactionDetails":             rpc.TransactionDetailsFull,
	"maxSupportedTransactionVersion": uint64(0),
	"rewards":                        false,
}

func (d *ExtServerDimension) getSlot(ctx context.Context, src string, sn uint64) (*Slot, error) {
	var raw *rawParsedBlock
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("%s/%d", src, sn),
		func(ctx context.Context, cli *Client) (res clientpool.Result) {
			res = cli.CallContext(ctx, &raw, src, "getBlock", sn, getParsedBlockOpts)
			if isSlotSkippedError(res.Err) {
				raw, res = nil, clientpool.Result{}
			}
			res.BrokenForTask = res.Err != nil // always retry using other client
			return res
		},
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get parsed block %d (%s) failed", sn, r.ConfigName)
	}
	if raw == nil {
		// slot was skipped
		return &Slot{SlotNumber: sn, Skipped: true}, nil
	}
	return &Slot{
		SlotNumber:        sn,
		Blockhash:         raw.Blockhash,
		PreviousBlockhash: raw.PreviousBlockhash,
		ParentSlot:        raw.ParentSlot,
		BlockHeight:       raw.BlockHeight,
		BlockTime:         raw.BlockTime,
		Transactions:      raw.Transactions,
	}, nil
}
