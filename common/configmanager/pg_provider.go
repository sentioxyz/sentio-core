package configmanager

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type SentioConfig struct {
	Key       string `gorm:"primaryKey"`
	Data      []byte
	UpdatedAt time.Time
}

func (SentioConfig) TableName() string {
	return "_sentio_configs"
}

const (
	kTableOptID = iota
	kKeyOptID
	kEncoderOptID
	kDisableAutoMigrationOptID
)

type PgProviderOption struct {
	optID                int
	table                string
	keyColumn            string
	dataColumn           string
	key                  string
	encoder              ConfigEncoder
	disableAutoMigration bool
}

func WithPgTable(table, keyColumn, dataColumn string) PgProviderOption {
	return PgProviderOption{optID: kTableOptID, table: table, keyColumn: keyColumn, dataColumn: dataColumn}
}

func WithPgKey(key string) PgProviderOption {
	return PgProviderOption{optID: kKeyOptID, key: key}
}

func WithPgEncoder(encoder ConfigEncoder) PgProviderOption {
	return PgProviderOption{optID: kEncoderOptID, encoder: encoder}
}

func WithPgDisableAutoMigration() PgProviderOption {
	return PgProviderOption{optID: kDisableAutoMigrationOptID, disableAutoMigration: true}
}

type PgProviderOptions []PgProviderOption

func (p PgProviderOptions) Merge() PgProviderOption {
	var final = PgProviderOption{
		table: SentioConfig{}.TableName(),
	}
	for _, opt := range p {
		switch opt.optID {
		case kTableOptID:
			final.table = opt.table
			final.dataColumn = opt.dataColumn
			final.keyColumn = opt.keyColumn
		case kKeyOptID:
			final.key = opt.key
		case kEncoderOptID:
			final.encoder = opt.encoder
		case kDisableAutoMigrationOptID:
			final.disableAutoMigration = opt.disableAutoMigration
		}
	}
	return final
}

type PgProvider struct {
	db     *gorm.DB
	option PgProviderOption

	watching bool
	lastData []byte
	stopCh   chan struct{}
	mu       sync.Mutex
}

func NewPgProvider(db *gorm.DB, options ...PgProviderOption) koanf.Provider {
	option := PgProviderOptions{}
	option = append(option, options...)
	o := option.Merge()
	if !o.disableAutoMigration {
		_ = db.AutoMigrate(&SentioConfig{})
	}
	return &PgProvider{db: db, option: o}
}

func (p *PgProvider) ReadBytes() ([]byte, error) {
	switch {
	case p.option.key == "":
		return nil, fmt.Errorf("unspecified key")
	case p.option.table == SentioConfig{}.TableName():
		c, err := gorm.G[SentioConfig](p.db).Where("key = ?", p.option.key).First(context.Background())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read config from table %s:%s", p.option.table, p.option.key)
		}
		return c.Data, nil
	default:
		rows, err := p.db.Where(gorm.Expr(fmt.Sprintf("%s = ?", p.option.keyColumn), p.option.key)).Select(p.option.dataColumn).Rows()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read custom config from table %s:%s", p.option.table, p.option.key)
		}
		defer func() {
			_ = rows.Close()
		}()
		for rows.Next() {
			var data []byte
			if err := rows.Scan(&data); err != nil {
				return nil, errors.Wrapf(err, "failed to scan config data from table %s:%s", p.option.table, p.option.key)
			}
			return data, nil
		}
		return nil, errors.Errorf("config not found in table %s:%s", p.option.table, p.option.key)
	}
}

func (p *PgProvider) Read() (map[string]any, error) {
	switch {
	case p.option.encoder == "":
		return nil, fmt.Errorf("unspecified encoder")
	default:
		if b, err := p.ReadBytes(); err != nil {
			return nil, err
		} else {
			return p.option.encoder.Parse(b)
		}
	}
}

// Watch polls Postgres at the given period. When the underlying data changes,
// it calls cb with the latest bytes in the event parameter.
func (p *PgProvider) Watch(period time.Duration, cb func(body any, err error)) {
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

func (p *PgProvider) Unwatch() {
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
