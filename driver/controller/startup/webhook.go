package startup

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/service/processor/models"
)

type webhookController struct {
	processor *models.Processor
	topic     *pubsub.Topic

	mu        sync.Mutex
	cached    map[uint64]map[uint64][]controller.WebhookMessage // map[<blockNumber>][<taskIndex>]
	committed *uint64
}

func newWebhookController(processor *models.Processor, topic *pubsub.Topic) *webhookController {
	return &webhookController{
		processor: processor,
		topic:     topic,
		cached:    make(map[uint64]map[uint64][]controller.WebhookMessage),
	}
}

func (c *webhookController) Reset(ctx context.Context, checkpoint *controller.Checkpoint) *controller.ExternalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	if checkpoint == nil {
		c.cached = make(map[uint64]map[uint64][]controller.WebhookMessage)
	} else {
		utils.MapDelete(c.cached, func(bn uint64) bool {
			return bn > checkpoint.BlockNumber
		})
	}
	// sent msg cannot be canceled
	return nil
}

type SingleWebhookMessage struct {
	ExportName      string `json:"export_name"`
	EventID         uint64 `json:"event_id"`
	TimestampMicros uint64 `json:"timestamp_micros"`
	Version         uint64 `json:"version"`
	Data            string `json:"data"`
}

const MaxMessagesPerBlock = 10000000

var maxUncommitedWebhookMessages = envconf.LoadUInt64("SENTIO_MAX_UNCOMMITED_WEBHOOK_MESSAGES", 1000000,
	envconf.WithMin(10000), envconf.WithMax(1000000))

func (c *webhookController) getCachedSize(blockNumber uint64) (total uint64) {
	for bn, blockMsgs := range c.cached {
		if bn > blockNumber {
			continue
		}
		for _, msgs := range blockMsgs {
			total += uint64(len(msgs))
		}
	}
	return total
}

func (c *webhookController) CachedTooMuch(blockNumber uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getCachedSize(blockNumber) > maxUncommitedWebhookMessages
}

func (c *webhookController) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (stat map[string]int, extErr *controller.ExternalError) {
	// build SingleWebhookMessage from c.cached into dict
	stat = make(map[string]int)
	dict := make(map[string][]SingleWebhookMessage)
	c.mu.Lock()
	for _, bn := range utils.GetOrderedMapKeys(c.cached) {
		if bn > blockNumber {
			continue
		}
		blockMsgs := c.cached[bn]
		if count := utils.CountMap(blockMsgs); count >= MaxMessagesPerBlock {
			c.mu.Unlock()
			return nil, controller.NewExternalError(controller.ErrCodeTooManyWebhookMsgEntity,
				errors.Errorf("too many messages in block %d, %d > %d", bn, count, MaxMessagesPerBlock))
		}
		var blockSeq uint64
		for _, messages := range utils.GetMapValuesOrderByKey(blockMsgs) {
			for _, msg := range messages {
				blockSeq++
				dict[msg.Channel] = append(dict[msg.Channel], SingleWebhookMessage{
					ExportName:      msg.Name,
					EventID:         bn*MaxMessagesPerBlock + blockSeq,
					TimestampMicros: uint64(msg.BlockTime.UnixMicro()),
					Version:         uint64(c.processor.Version),
					Data:            msg.Payload,
				})
				stat[msg.Name] += 1
			}
		}
	}
	c.mu.Unlock()

	// actually send SingleWebhookMessage
	g, gctx := errgroup.WithContext(ctx)
	for channel, messages := range dict {
		data, err := json.Marshal(messages)
		if err != nil {
			panic(errors.Wrapf(err, "json marshal message for channel %s failed", channel))
		}
		pubSubMsg := &pubsub.Message{
			Data: data,
			Attributes: map[string]string{
				"channel_name": channel,
				"project_id":   c.processor.ProjectID,
				"processor_id": c.processor.ID,
			},
		}
		g.Go(func() error {
			_, pubErr := c.topic.Publish(gctx, pubSubMsg).Get(gctx)
			if pubErr != nil {
				return errors.Wrapf(pubErr, "publish message for channel %s failed", channel)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, controller.NewExternalError(controller.ErrCodeSendWebhookDataFailed, err)
	}

	// send succeed, clean c.cached
	c.mu.Lock()
	defer c.mu.Unlock()
	utils.MapDelete(c.cached, func(bn uint64) bool {
		return bn <= blockNumber
	})
	c.committed = &blockNumber
	return
}

func (c *webhookController) Insert(
	blockNumber uint64,
	taskIndex controller.TaskIndex,
	messages []controller.WebhookMessage,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	org, _ := utils.GetFromK2Map(c.cached, blockNumber, taskIndex.Global)
	utils.PutIntoK2Map(c.cached, blockNumber, taskIndex.Global, append(org, messages...))
}

func (c *webhookController) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"committed":       c.committed,
		"uncommitedTotal": c.getCachedSize(math.MaxUint64),
		"uncommited": cacheSnapshot(c.cached, func(msgs []controller.WebhookMessage) (s int) {
			return len(msgs)
		}),
	}
}
