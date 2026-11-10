package models

import (
	"testing"

	"sentioxyz/sentio-core/service/processor/protos"

	"github.com/stretchr/testify/assert"
)

func TestCanResumePause(t *testing.T) {
	kinds := []ProcessorReasonKind{
		"", ProcessorReasonKindBilling, ProcessorReasonKindSecurity, ProcessorReasonKindAccountSuspended, "bogus",
	}
	allowed := map[[2]ProcessorReasonKind]bool{
		// a kindless pause (user pause or entry recorded before kinds existed)
		// is resumable by anything
		{"", ""}:                                  true,
		{"", ProcessorReasonKindBilling}:          true,
		{"", ProcessorReasonKindSecurity}:         true,
		{"", ProcessorReasonKindAccountSuspended}: true,
		// a billing pause also accepts an unspecified resume
		{ProcessorReasonKindBilling, ""}:                          true,
		{ProcessorReasonKindBilling, ProcessorReasonKindBilling}:  true,
		{ProcessorReasonKindBilling, ProcessorReasonKindSecurity}: false,
		// a security pause is only resumable by a security resume
		{ProcessorReasonKindSecurity, ""}:                          false,
		{ProcessorReasonKindSecurity, ProcessorReasonKindBilling}:  false,
		{ProcessorReasonKindSecurity, ProcessorReasonKindSecurity}: true,
		// an account-suspended pause is only resumable by an account-suspended
		// resume (the system, after it confirmed the account is active again)
		{ProcessorReasonKindAccountSuspended, ProcessorReasonKindAccountSuspended}: true,
		// every other pair is rejected, including any pair involving a kind this
		// code does not know (e.g. one written by newer code)
	}
	for _, pauseKind := range kinds {
		for _, resumeKind := range kinds {
			assert.Equalf(t, allowed[[2]ProcessorReasonKind{pauseKind, resumeKind}],
				CanResumePause(pauseKind, resumeKind),
				"pause kind %q, resume kind %q", pauseKind, resumeKind)
		}
	}
}

func TestReasonKindPBConversion(t *testing.T) {
	// model -> proto -> model round trip
	for _, kind := range []ProcessorReasonKind{
		"", ProcessorReasonKindBilling, ProcessorReasonKindSecurity, ProcessorReasonKindAccountSuspended,
	} {
		assert.Equal(t, kind, ReasonKindFromPB(kind.ToPB()), "round trip of %q", kind)
	}
	// proto -> model -> proto round trip
	for pb := range protos.ReasonKind_name {
		kind := protos.ReasonKind(pb)
		assert.Equal(t, kind, ReasonKindFromPB(kind).ToPB(), "round trip of %v", kind)
	}
	// unknown model values map to unspecified rather than panicking
	assert.Equal(t, protos.ReasonKind_UNSPECIFIED, ProcessorReasonKind("bogus").ToPB())
}
