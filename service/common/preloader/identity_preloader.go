package preloader

import (
	"context"
	commonmodels "sentioxyz/sentio-core/service/common/models"
)

func PreLoadedIdentity(ctx context.Context) *commonmodels.Identity {
	if identity, ok := ctx.Value("identity").(*commonmodels.Identity); ok {
		return identity
	}
	return nil // Return nil if no identity is found in the context
}
