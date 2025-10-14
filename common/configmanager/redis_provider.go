package configmanager

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

const RedisDefaultCategory string = "_sentio_configs"

type RedisProviderOption struct {
	category string
	key      string
	encoder  ConfigEncoder
}

type RedisProviderOptions []RedisProviderOption

func (o RedisProviderOptions) Merge() RedisProviderOption {
	var res = RedisProviderOption{
		category: RedisDefaultCategory,
	}
	for _, opt := range o {
		if opt.category != "" {
			res.category = opt.category
		}
		if opt.key != "" {
			res.key = opt.key
		}
		if opt.encoder != "" {
			res.encoder = opt.encoder
		}
	}
	return res
}

func WithRedisKey(key string) RedisProviderOption { return RedisProviderOption{key: key} }
func WithRedisCategory(category string) RedisProviderOption {
	return RedisProviderOption{category: category}
}
func WithRedisEncoder(encoder ConfigEncoder) RedisProviderOption {
	return RedisProviderOption{encoder: encoder}
}

type RedisProvider struct {
	cli    *redis.Client
	option RedisProviderOption

	watching bool
	lastData []byte
	stopCh   chan struct{}
	mu       sync.Mutex
}

func NewRedisProvider(cli *redis.Client, options ...RedisProviderOption) koanf.Provider {
	option := RedisProviderOptions{}
	option = append(option, options...)
	return &RedisProvider{cli: cli, option: option.Merge()}
}

func (p *RedisProvider) ReadBytes() ([]byte, error) {
	switch {
	case p.option.key == "":
		return nil, errors.Errorf("unspecified key")
	default:
		b, err := p.cli.HGet(context.Background(), p.option.category, p.option.key).Bytes()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read config from redis:%s:%s", p.option.category, p.option.key)
		}
		return b, nil
	}
}

func (p *RedisProvider) Read() (map[string]any, error) {
	switch {
	case p.option.encoder == "":
		return nil, errors.Errorf("unspecified encoder")
	default:
		b, err := p.ReadBytes()
		if err != nil {
			return nil, err
		}
		return p.option.encoder.Parse(b)
	}
}

// Watch polls Redis at the given period and invokes cb when the value changes.
func (p *RedisProvider) Watch(period time.Duration, cb func(body any, err error)) {
	if cb == nil {
		return
	}
	if period <= 0 {
		period = 5 * time.Second
	}

	p.mu.Lock()
	if p.watching {
		p.mu.Unlock()
		return
	}
	p.stopCh = make(chan struct{})
	stopCh := p.stopCh
	p.watching = true
	p.mu.Unlock()

	// Initial snapshot outside lock; don't trigger cb on baseline load.
	initBytes, err := p.ReadBytes()
	if err != nil {
		cb(nil, err)
	}

	p.mu.Lock()
	if err == nil {
		p.lastData = append([]byte(nil), initBytes...)
	} else {
		p.lastData = nil
	}
	p.mu.Unlock()

	go func() {
		defer func() { _ = recover() }()
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				b, err := p.ReadBytes()
				if err != nil {
					cb(nil, err)
					continue
				}

				p.mu.Lock()
				changed := !bytes.Equal(b, p.lastData)
				if changed {
					p.lastData = append([]byte(nil), b...)
				}
				p.mu.Unlock()

				if changed {
					cb(string(b), nil)
				}
			}
		}
	}()
}

func (p *RedisProvider) Unwatch() {
	p.mu.Lock()
	if !p.watching {
		p.mu.Unlock()
		return
	}
	close(p.stopCh)
	p.stopCh = nil
	p.watching = false
	p.mu.Unlock()
}
