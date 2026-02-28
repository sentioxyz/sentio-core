package functions

import (
	"context"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
)

type Handler interface {
	Handle(matrix *protos.Matrix) (*protos.Matrix, error)
	Category() string
}

func Process(ctx context.Context,
	matrix *protos.Matrix, funcs []*protos.Function) (*protos.Matrix, error) {
	if len(funcs) == 0 {
		return matrix, nil
	}

	var err error
	categoryProcessed := map[string]bool{}
	for _, f := range funcs {
		var h Handler
		switch f.Name {
		case "bottomk":
			h = NewBottomKHandler(f.Arguments)
		case "topk":
			h = NewTopKHandler(f.Arguments)
		case "delta":
			h = NewDeltaHandler(f.Arguments)
		case "truncate":
			h = NewTruncateHandler(f.Arguments)
		default:
			h = nil
		}
		if h == nil {
			log.WithContext(ctx).Infof("function name %s not found", f.Name)
			continue
		}

		if categoryProcessed[h.Category()] {
			if f.Name == "truncate" {
				// truncate function can be processed multiple times
				continue
			}
			log.WithContext(ctx).Warnf("category %s already processed", h.Category())
			return nil, errors.Errorf("%s function should be only processed once", h.Category())
		}
		categoryProcessed[h.Category()] = true
		matrix, err = h.Handle(matrix)
		if err != nil {
			log.WithContext(ctx).Warnf("failed to process function %s: %v", f.Name, err)
			return nil, err
		}
	}
	return matrix, nil
}
