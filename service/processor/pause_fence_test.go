package processor

import (
	"testing"

	"sentioxyz/sentio-core/service/processor/models"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVerifyPauseFence(t *testing.T) {
	pause := func(id string, kind models.ProcessorReasonKind) models.ProcessorStateHistory {
		return models.ProcessorStateHistory{ID: id, Action: models.ProcessorStateActionPause, Kind: kind}
	}
	resume := func(id string) models.ProcessorStateHistory {
		return models.ProcessorStateHistory{ID: id, Action: models.ProcessorStateActionResume}
	}
	active := func(id string) models.ProcessorStateHistory {
		return models.ProcessorStateHistory{ID: id, Action: models.ProcessorStateActionActive}
	}

	tests := []struct {
		name      string
		histories []models.ProcessorStateHistory // newest first
		fenceID   string
		kind      models.ProcessorReasonKind
		wantErr   string
	}{
		{
			name:    "no history",
			fenceID: "p1",
			wantErr: "pause state changed",
		},
		{
			name:      "latest pause matches fence",
			histories: []models.ProcessorStateHistory{pause("p1", models.ProcessorReasonKindBilling)},
			fenceID:   "p1",
			kind:      models.ProcessorReasonKindBilling,
		},
		{
			name: "active and obsolete entries are skipped",
			histories: []models.ProcessorStateHistory{
				active("a1"), pause("p1", models.ProcessorReasonKindBilling), resume("r1"),
			},
			fenceID: "p1",
			kind:    models.ProcessorReasonKindBilling,
		},
		{
			name:      "latest pause is a different one",
			histories: []models.ProcessorStateHistory{pause("p2", models.ProcessorReasonKindBilling), pause("p1", "")},
			fenceID:   "p1",
			kind:      models.ProcessorReasonKindBilling,
			wantErr:   "pause state changed",
		},
		{
			name:      "resumed since observed",
			histories: []models.ProcessorStateHistory{resume("r1"), pause("p1", models.ProcessorReasonKindBilling)},
			fenceID:   "p1",
			kind:      models.ProcessorReasonKindBilling,
			wantErr:   "pause state changed",
		},
		{
			name:      "billing pause accepts unspecified resume",
			histories: []models.ProcessorStateHistory{pause("p1", models.ProcessorReasonKindBilling)},
			fenceID:   "p1",
			kind:      "",
		},
		{
			name:      "security pause rejects unspecified resume",
			histories: []models.ProcessorStateHistory{pause("p1", models.ProcessorReasonKindSecurity)},
			fenceID:   "p1",
			kind:      "",
			wantErr:   "not resumable",
		},
		{
			name:      "security pause rejects billing resume",
			histories: []models.ProcessorStateHistory{pause("p1", models.ProcessorReasonKindSecurity)},
			fenceID:   "p1",
			kind:      models.ProcessorReasonKindBilling,
			wantErr:   "not resumable",
		},
		{
			name:      "security pause accepts security resume",
			histories: []models.ProcessorStateHistory{pause("p1", models.ProcessorReasonKindSecurity)},
			fenceID:   "p1",
			kind:      models.ProcessorReasonKindSecurity,
		},
		{
			name:      "legacy pause without kind accepts any resume kind",
			histories: []models.ProcessorStateHistory{pause("p1", "")},
			fenceID:   "p1",
			kind:      models.ProcessorReasonKindBilling,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyPauseFence(tt.histories, tt.fenceID, tt.kind)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
			assert.Equal(t, codes.FailedPrecondition, status.Code(err))
		})
	}
}
