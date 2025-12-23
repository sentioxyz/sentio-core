package timerange

import (
	"context"
	"encoding/json"
	"time"
	_ "time/tzdata"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/sqlbuilder/condition"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/huandu/go-sqlbuilder"
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

func (t *TimeRange) SetClickhouseTimeSelector(s *sqlbuilder.SelectBuilder, timeColumnName string) {
	if t == nil {
		return
	}
	var conditions []string
	if t.RangeMode == LeftOpenRange || t.RangeMode == BothOpenRange {
		conditions = append(conditions, s.GreaterThan(timeColumnName, t.Start.In(t.Timezone)))
	} else {
		conditions = append(conditions, s.GreaterEqualThan(timeColumnName, t.Start.In(t.Timezone)))
	}
	if t.RangeMode == RightOpenRange || t.RangeMode == BothOpenRange {
		conditions = append(conditions, s.LessThan(timeColumnName, t.End.In(t.Timezone)))
	} else {
		conditions = append(conditions, s.LessEqualThan(timeColumnName, t.End.In(t.Timezone)))
	}
	s.Where(conditions...)
}

func (t *TimeRange) BuildTimeConditions(timeColumnName string) []*condition.Cond {
	if t == nil {
		return nil
	}
	var conditions []*condition.Cond
	if t.RangeMode == LeftOpenRange || t.RangeMode == BothOpenRange {
		conditions = append(conditions, condition.GreaterThan(timeColumnName, t.Start.In(t.Timezone)))
	} else {
		conditions = append(conditions, condition.GreaterEqualThan(timeColumnName, t.Start.In(t.Timezone)))
	}
	if t.RangeMode == RightOpenRange || t.RangeMode == BothOpenRange {
		conditions = append(conditions, condition.LessThan(timeColumnName, t.End.In(t.Timezone)))
	} else {
		conditions = append(conditions, condition.LessEqualThan(timeColumnName, t.End.In(t.Timezone)))
	}
	return conditions
}

func (t *TimeRange) ReverseStartCondition(timeColumnName string) *condition.Cond {
	if t == nil {
		return nil
	}
	switch t.RangeMode {
	case LeftOpenRange, BothOpenRange:
		return condition.LessEqualThan(timeColumnName, t.Start.In(t.Timezone))
	default:
		return condition.LessThan(timeColumnName, t.Start.In(t.Timezone))
	}
}
