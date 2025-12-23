package timerange

import (
	"context"
	"encoding/json"
	"time"
	_ "time/tzdata"
	
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrTimeRangeNil      = errors.Errorf("time range is nil")
	ErrTimeStartAfterEnd = errors.Errorf("time start is equal or after end")
)

const (
	ClosedRange    = iota
	LeftOpenRange  = 1
	RightOpenRange = 2
	BothOpenRange  = 3
)

type TimeRange struct {
	Start     time.Time
	End       time.Time
	Step      time.Duration
	Timezone  *time.Location
	RangeMode int
}

func NewTimeRangeFromLite(ctx context.Context, timeRange *protos.TimeRangeLite) (*TimeRange, error) {
	if timeRange == nil {
		return nil, status.Errorf(codes.InvalidArgument, ErrTimeRangeNil.Error())
	}
	var err error
	t := &TimeRange{
		Timezone:  time.UTC,
		RangeMode: ClosedRange,
	}

	if len(timeRange.GetTimezone()) > 0 {
		if t.Timezone, err = time.LoadLocation(timeRange.GetTimezone()); err != nil {
			log.WithContext(ctx).Infof("failed to load timezone %s, err: %v", timeRange.GetTimezone(), err)
			t.Timezone = time.UTC
		}
	}
	if t.Start, err = ResolveTimeStrWithAlign(timeRange.GetStart(), true, t.Timezone); err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			errors.Wrapf(err, "failed to resolve start time, err: %v", err).Error())
	}
	if t.End, err = ResolveTimeStrWithAlign(timeRange.GetEnd(), false, t.Timezone); err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			errors.Wrapf(err, "failed to resolve end time, err: %v", err).Error())
	}
	if t.Start.After(t.End) || t.Start.Equal(t.End) {
		return nil, status.Errorf(codes.InvalidArgument,
			ErrTimeStartAfterEnd.Error())
	}
	if timeRange.GetStep() < 0 {
		return nil, status.Errorf(codes.InvalidArgument,
			errors.Errorf("step must be positive, got %d", timeRange.GetStep()).Error())
	}
	t.Step = time.Duration(timeRange.GetStep()) * time.Second
	minStep := GetEventMinStep(t.Start, t.End)
	if t.Step < minStep {
		t.Step = minStep
	}

	return t, nil
}

func NewTimeRangeFromProto(ctx context.Context, timeRange *protos.TimeRange) (*TimeRange, error) {
	if timeRange == nil {
		return nil, status.Errorf(codes.InvalidArgument, ErrTimeRangeNil.Error())
	}
	var err error
	t := &TimeRange{
		Timezone: time.UTC,
	}
	if len(timeRange.GetTimezone()) > 0 {
		if t.Timezone, err = time.LoadLocation(timeRange.GetTimezone()); err != nil {
			log.WithContext(ctx).Infof("failed to load timezone %s, err: %v", timeRange.GetTimezone(), err)
			t.Timezone = time.UTC
		}
	}
	if t.Start, err = ResolveTimeLike(timeRange.GetStart(), true, t.Timezone); err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			errors.Wrapf(err, "failed to resolve start time, err: %v", err).Error())
	}
	if t.End, err = ResolveTimeLike(timeRange.GetEnd(), false, t.Timezone); err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			errors.Wrapf(err, "failed to resolve end time, err: %v", err).Error())
	}
	if t.Start.After(t.End) || t.Start.Equal(t.End) {
		return nil, status.Errorf(codes.InvalidArgument,
			ErrTimeStartAfterEnd.Error())
	}
	if timeRange.GetStep() > 0 {
		t.Step = time.Duration(timeRange.GetStep()) * time.Second
	} else if timeRange.GetInterval() != nil {
		switch timeRange.GetInterval().GetUnit() {
		case "seconds":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Second
		case "minutes":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Minute
		case "hours":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Hour
		case "days":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Hour * 24
		case "weeks":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Hour * 24 * 7
		case "months":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Hour * 24 * 30
		case "years":
			t.Step = time.Duration(timeRange.GetInterval().GetValue()) * time.Hour * 24 * 365
		}
	}

	return t, nil
}

func (t *TimeRange) String() string {
	str, err := json.Marshal(t)
	if err != nil {
		return err.Error()
	}
	return string(str)
}
