package chx

import (
	"fmt"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type Engine interface {
	Name() string
	Full() string
	Replicated() bool
}

type engineMergeTree struct {
	ReplicatedArgs []string
}

func (e engineMergeTree) Name() string {
	if len(e.ReplicatedArgs) == 0 {
		return "MergeTree"
	}
	return "ReplicatedMergeTree"
}

func (e engineMergeTree) Full() string {
	if len(e.ReplicatedArgs) == 0 {
		return "MergeTree()"
	}
	return fmt.Sprintf("ReplicatedMergeTree('%s')", strings.Join(e.ReplicatedArgs, "','"))
}

func (c engineMergeTree) Replicated() bool {
	return len(c.ReplicatedArgs) > 0
}

type engineVersionedCollapsingMergeTree struct {
	engineMergeTree

	SignFieldName    string
	VersionFieldName string
}

func (e engineVersionedCollapsingMergeTree) Name() string {
	if len(e.ReplicatedArgs) == 0 {
		return "VersionedCollapsingMergeTree"
	}
	return "ReplicatedVersionedCollapsingMergeTree"
}

func (e engineVersionedCollapsingMergeTree) Full() string {
	if len(e.ReplicatedArgs) == 0 {
		return fmt.Sprintf("VersionedCollapsingMergeTree(`%s`,`%s`)", e.SignFieldName, e.VersionFieldName)
	}
	return fmt.Sprintf("ReplicatedVersionedCollapsingMergeTree('%s',`%s`,`%s`)",
		strings.Join(e.ReplicatedArgs, "','"), e.SignFieldName, e.VersionFieldName)
}

func (c engineVersionedCollapsingMergeTree) Replicated() bool {
	return len(c.ReplicatedArgs) > 0
}

func buildEngineFromString(str string) (Engine, error) {
	engine, args, _ := strings.Cut(str, "(")
	args = strings.TrimSuffix(strings.TrimPrefix(args, "("), ")")
	params := utils.MapSliceNoError(strings.Split(args, ","), strings.TrimSpace)
	switch engine {
	case "MergeTree":
		if args != "" {
			return nil, errors.Errorf("invalid MergeTree arguments: %s", str)
		}
		return engineMergeTree{}, nil
	case "ReplicatedMergeTree":
		if len(params) != 2 {
			return nil, errors.Errorf("invalid ReplicatedMergeTree arguments: %s", str)
		}
		return engineMergeTree{
			ReplicatedArgs: utils.MapSliceNoError(params, func(arg string) string {
				return strings.Trim(arg, "'")
			}),
		}, nil
	case "VersionedCollapsingMergeTree":
		if len(params) != 2 {
			return nil, errors.Errorf("invalid VersionedCollapsingMergeTree arguments: %s", str)
		}
		return engineVersionedCollapsingMergeTree{
			SignFieldName:    strings.Trim(params[0], "`"),
			VersionFieldName: strings.Trim(params[1], "`"),
		}, nil
	case "ReplicatedVersionedCollapsingMergeTree":
		if len(params) != 4 {
			return nil, errors.Errorf("invalid VersionedCollapsingMergeTree arguments: %s", str)
		}
		return engineVersionedCollapsingMergeTree{
			engineMergeTree: engineMergeTree{
				ReplicatedArgs: utils.MapSliceNoError(params[:2], func(arg string) string {
					return strings.Trim(arg, "'")
				}),
			},
			SignFieldName:    strings.Trim(params[2], "`"),
			VersionFieldName: strings.Trim(params[3], "`"),
		}, nil
	default:
		return nil, errors.Errorf("unknown engine %q", str)
	}
}

var DefaultReplicatedArgs = []string{"/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}", "{replica}"}

func newDefaultMergeTreeEngine(onCluster bool) engineMergeTree {
	e := engineMergeTree{}
	if onCluster {
		e.ReplicatedArgs = DefaultReplicatedArgs
	}
	return e
}

func NewDefaultMergeTreeEngine(onCluster bool) Engine {
	return newDefaultMergeTreeEngine(onCluster)
}

func NewDefaultVersionedCollapsingMergeTreeEngine(
	onCluster bool,
	signFieldName string,
	versionFieldName string,
) Engine {
	e := engineVersionedCollapsingMergeTree{
		engineMergeTree:  newDefaultMergeTreeEngine(onCluster),
		SignFieldName:    signFieldName,
		VersionFieldName: versionFieldName,
	}
	return e
}
