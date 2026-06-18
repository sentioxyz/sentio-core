package controller

import (
	"context"
	"time"
)

type WebhookController interface {
	Reset(ctx context.Context, checkpoint *Checkpoint) *ExternalError
	CachedTooMuch(blockNumber uint64) bool
	Commit(ctx context.Context, blockNumber uint64, blockTime time.Time) (stat map[string]int, err *ExternalError)
	Insert(blockNumber uint64, taskIndex TaskIndex, messages []WebhookMessage)
	Snapshot() any
}

type WebhookMessage struct {
	Name      string
	Channel   string
	BlockTime time.Time
	Payload   string
}

func StatisticWebhookMessages(msgs []WebhookMessage, stat map[string]int) {
	for _, msg := range msgs {
		stat[msg.Name] += 1
	}
}

type EmptyWebhookController struct{}

func (c EmptyWebhookController) Reset(ctx context.Context, checkpoint *Checkpoint) *ExternalError {
	return nil
}

func (c EmptyWebhookController) CachedTooMuch(blockNumber uint64) bool {
	return false
}

func (c EmptyWebhookController) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (map[string]int, *ExternalError) {
	return nil, nil
}

func (c EmptyWebhookController) Insert(blockNumber uint64, taskIndex TaskIndex, messages []WebhookMessage) {
	if len(messages) > 0 {
		panic("do not support webhook data")
	}
}

func (c EmptyWebhookController) Snapshot() any {
	return nil
}
