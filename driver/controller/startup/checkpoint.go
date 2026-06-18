package startup

import (
	"context"
	"encoding/json"
	"math"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/compress"
	"sentioxyz/sentio-core/driver/controller"
	sentioerror "sentioxyz/sentio-core/service/common/errors"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/processor/protos"
)

type checkpointStore struct {
	processor        *models.Processor
	chainID          string
	cli              protos.ProcessorServiceClient
	chainState       *models.ChainState
	latestCheckpoint *controller.Checkpoint
}

func newCheckpointStore(
	processor *models.Processor,
	chainID string,
	cli protos.ProcessorServiceClient,
	chainState *models.ChainState,
) *checkpointStore {
	return &checkpointStore{
		processor:  processor,
		chainID:    chainID,
		cli:        cli,
		chainState: chainState,
	}
}

func LoadCheckpoints(cs *models.ChainState) (checkpoints []controller.Checkpoint, err error) {
	if len(cs.IndexerState) == 0 {
		return nil, nil
	}
	if err = compress.Load(cs.IndexerState, &checkpoints); err != nil {
		return nil, errors.Wrap(err, "unmarshal checkpoints failed")
	}
	return checkpoints, nil
}

func LoadTemplates(cs *models.ChainState) (templates map[uint64][]controller.TemplateInstance, err error) {
	if len(cs.Templates) == 0 {
		return make(map[uint64][]controller.TemplateInstance), nil
	}
	if err = compress.Load([]byte(cs.Templates), &templates); err != nil {
		return nil, errors.Wrapf(err, "unmarshal templates for chain %s failed", cs.ChainID)
	}
	return templates, nil
}

func SetCheckpoints(cs *models.ChainState, checkpoints []controller.Checkpoint) error {
	// set indexer_state
	b, err := compress.Dump(checkpoints)
	if err != nil {
		return errors.Wrapf(err, "dump checkpoints failed")
	}
	if len(checkpoints) > 2 && len(b) > maxIndexerStateSize {
		return errors.Wrapf(controller.ErrCheckpointsTooBig,
			"length of IndexerState in %d checkpoints is %d, over the limit %d",
			len(checkpoints), len(b), maxIndexerStateSize)
	}
	cs.IndexerState = b
	// set progress and state
	cs.ProcessedBlockNumber = -1
	cs.ProcessedBlockHash = ""
	cs.ProcessedTimestampMicros = 0
	cs.InitialStartBlockNumber = 0
	cs.EstimatedLatestBlockNumber = math.MaxInt64
	cs.LastBlockNumber = 0
	cs.State = int32(protos.ChainState_Status_CATCHING_UP)
	if len(checkpoints) > 0 {
		last := checkpoints[len(checkpoints)-1]
		cs.ProcessedBlockNumber = int64(last.BlockNumber)
		cs.ProcessedBlockHash = last.BlockHash
		cs.ProcessedTimestampMicros = last.BlockTime.UnixMicro()
		cs.InitialStartBlockNumber = int64(last.FullBlockRange.StartBlock)
		cs.EstimatedLatestBlockNumber = int64(last.CurrentLastBlockNumber())
		if last.FullBlockRange.EndBlock != nil {
			cs.LastBlockNumber = int64(*last.FullBlockRange.EndBlock)
		}
		if last.InWatching() || last.AllDone() {
			cs.State = int32(protos.ChainState_Status_PROCESSING_LATEST)
		}
	}
	// remove error
	cs.ErrorRecord = sentioerror.ErrorRecord{}
	return nil
}

func SetTemplates(cs *models.ChainState, templates map[uint64][]controller.TemplateInstance) error {
	b, err := compress.Dump(templates)
	if err != nil {
		return errors.Wrapf(err, "dump templates failed")
	}
	cs.Templates = string(b)
	return nil
}

func SetHandlerStat(cs *models.ChainState, agentStat map[string]int) error {
	b, err := json.Marshal(agentStat)
	if err != nil {
		return errors.Wrapf(err, "dump agent stat failed")
	}
	cs.HandlerStat = b
	return nil
}

func (s *checkpointStore) Load(ctx context.Context) (
	checkpoints []controller.Checkpoint,
	templates map[uint64][]controller.TemplateInstance,
	err error,
) {
	if checkpoints, err = LoadCheckpoints(s.chainState); err != nil {
		return nil, nil, err
	}
	if templates, err = LoadTemplates(s.chainState); err != nil {
		return nil, nil, err
	}
	if len(checkpoints) > 0 {
		s.latestCheckpoint = &checkpoints[len(checkpoints)-1]
	}
	return
}

const maxIndexerStateSize = 50 * 1024 * 1024 // 50MB

func (s *checkpointStore) Save(
	ctx context.Context,
	checkpoints []controller.Checkpoint,
	templates map[uint64][]controller.TemplateInstance,
	agentStat map[string]int,
) error {
	var latest *controller.Checkpoint
	if len(checkpoints) > 0 {
		latest = &checkpoints[len(checkpoints)-1]
	}
	reorg := (latest == nil && s.latestCheckpoint != nil) ||
		(latest != nil && s.latestCheckpoint != nil && latest.BlockNumber < s.latestCheckpoint.BlockNumber)
	var reorgBlocks uint64
	var reduceToBlock int64
	if reorg {
		if latest == nil {
			reorgBlocks = s.latestCheckpoint.BlockNumber - s.latestCheckpoint.FullBlockRange.StartBlock + 1
			reduceToBlock = -1
		} else {
			reorgBlocks = s.latestCheckpoint.BlockNumber - latest.BlockNumber
			reduceToBlock = int64(latest.BlockNumber)
		}
	}

	cs := *s.chainState
	if err := SetCheckpoints(&cs, checkpoints); err != nil {
		return err
	}
	if err := SetTemplates(&cs, templates); err != nil {
		return err
	}
	if err := SetHandlerStat(&cs, agentStat); err != nil {
		return err
	}

	if err := updateChainState(ctx, s.cli, cs); err != nil {
		return err
	}
	s.chainState = &cs
	s.latestCheckpoint = latest

	if reorg {
		controller.N.ReorgDone(ctx, s.processor, s.chainID, reorgBlocks, reduceToBlock)
	}
	return nil
}

func (s *checkpointStore) SaveError(ctx context.Context, extErr *controller.ExternalError) error {
	cs := *s.chainState
	cs.ErrorRecord = newErrorRecord(extErr)
	cs.State = int32(protos.ChainState_Status_ERROR)
	if err := updateChainState(ctx, s.cli, cs); err != nil {
		return err
	}
	s.chainState = &cs
	return nil
}
