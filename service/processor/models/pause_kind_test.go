package models

import (
	"testing"

	"sentioxyz/sentio-core/service/processor/protos"

	"github.com/stretchr/testify/assert"
)

func TestCanResumePause(t *testing.T) {
	kinds := []ProcessorPauseKind{"", ProcessorPauseKindBilling, ProcessorPauseKindSecurity}
	allowed := map[[2]ProcessorPauseKind]bool{
		// a kindless pause (user pause or entry recorded before kinds existed)
		// is resumable by anything
		{"", ""}:                         true,
		{"", ProcessorPauseKindBilling}:  true,
		{"", ProcessorPauseKindSecurity}: true,
		// a billing pause also accepts an unspecified resume
		{ProcessorPauseKindBilling, ""}:                         true,
		{ProcessorPauseKindBilling, ProcessorPauseKindBilling}:  true,
		{ProcessorPauseKindBilling, ProcessorPauseKindSecurity}: false,
		// a security pause is only resumable by a security resume
		{ProcessorPauseKindSecurity, ""}:                         false,
		{ProcessorPauseKindSecurity, ProcessorPauseKindBilling}:  false,
		{ProcessorPauseKindSecurity, ProcessorPauseKindSecurity}: true,
	}
	for _, pauseKind := range kinds {
		for _, resumeKind := range kinds {
			assert.Equalf(t, allowed[[2]ProcessorPauseKind{pauseKind, resumeKind}],
				CanResumePause(pauseKind, resumeKind),
				"pause kind %q, resume kind %q", pauseKind, resumeKind)
		}
	}
}

func TestPauseKindPBConversion(t *testing.T) {
	// model -> proto -> model round trip
	for _, kind := range []ProcessorPauseKind{"", ProcessorPauseKindBilling, ProcessorPauseKindSecurity} {
		assert.Equal(t, kind, PauseKindFromPB(kind.ToPB()), "round trip of %q", kind)
	}
	// proto -> model -> proto round trip
	for pb := range protos.PauseKind_name {
		kind := protos.PauseKind(pb)
		assert.Equal(t, kind, PauseKindFromPB(kind).ToPB(), "round trip of %v", kind)
	}
	// unknown model values map to unspecified rather than panicking
	assert.Equal(t, protos.PauseKind_PAUSE_KIND_UNSPECIFIED, ProcessorPauseKind("bogus").ToPB())
}
