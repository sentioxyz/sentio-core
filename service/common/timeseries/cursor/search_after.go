package cursor

import (
	"context"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

type Sort struct {
	Field string
	Desc  bool
}

type SearchAfter interface {
	GenCursor(ctx context.Context, limit int) error
	GetLimitOffset() (limit, offset int)
	GetSorts() []string
	GetAfter(ctx context.Context) string
}

type searchAfter struct {
	Cursor
	sorts     []Sort
	arguments []any
	client    *redis.Client
}

func CreateSearchAfter(client *redis.Client,
	sorts []Sort, arguments []*protoscommon.Any) SearchAfter {
	searchAfter := &searchAfter{
		client: client,
		sorts:  sorts,
	}
	lo.ForEach(arguments, func(v *protoscommon.Any, _ int) {
		searchAfter.arguments = append(searchAfter.arguments, utils.Proto2Any(v))
	})
	return searchAfter
}

func (s *searchAfter) GenCursor(ctx context.Context, limit int) error {
	if len(s.arguments) > 0 {
		cursorID, ok := s.arguments[0].(string)
		if !ok {
			return errors.Errorf("invalid cursor id")
		}
		cursorBody, err := s.client.Get(ctx, cursorID).Bytes()
		switch {
		case errors.Is(err, redis.Nil):
			return errors.Errorf("cursor not found: %s", cursorID)
		case err != nil:
			return errors.Errorf("get cursor failed, err: %v", err)
		default:
			cursor, err := LoadCursor(string(cursorBody))
			if err != nil {
				return err
			}
			s.Cursor = cursor
		}
	} else {
		s.Cursor = NewInfiniteCursorWithStep(limit)
	}
	return nil
}

func (s *searchAfter) GetLimitOffset() (limit, offset int) {
	limit = s.GetLimit()
	offset = s.GetOffset()
	return
}

func (s *searchAfter) GetSorts() []string {
	var sorts []string
	for _, sort := range s.sorts {
		if sort.Desc {
			sorts = append(sorts, sort.Field+" DESC")
		} else {
			sorts = append(sorts, sort.Field+" ASC")
		}
	}
	return sorts
}

func (s *searchAfter) GetAfter(ctx context.Context) string {
	if s.Cursor == nil {
		return ""
	}
	go func() {
		if err := s.client.Del(context.Background(), s.Cursor.Cursor()).Err(); err != nil {
			log.Warnf("delete cursor failed, err: %v", err)
		}
	}()

	cursor := s.Cursor.Next()
	if cursor != nil {
		if err := s.client.Set(ctx, cursor.Cursor(), cursor.Dump(), TTL).Err(); err != nil {
			log.Warnf("save cursor meta failed, err: %v", err)
			return ""
		}
		return cursor.Cursor()
	}
	return ""
}
