package clickhouse

import (
	"crypto/sha256"
	"fmt"
	"github.com/bytedance/sonic"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sort"
)

type metaAndTable struct {
	meta  timeseries.Meta
	table chx.Table
}

type storeMeta []metaAndTable

func newStoreMeta(items []metaAndTable) storeMeta {
	sort.Slice(items, func(i, j int) bool {
		if items[i].meta.Type != items[j].meta.Type {
			return items[i].meta.Type < items[j].meta.Type
		}
		return items[i].meta.Name < items[j].meta.Name
	})
	return items
}

func (s storeMeta) add(meta timeseries.Meta, table chx.Table) storeMeta {
	return newStoreMeta(append(s, metaAndTable{meta: meta, table: table}))
}

func (s storeMeta) set(meta timeseries.Meta, table chx.Table) bool {
	for i, item := range s {
		if item.meta.Type == meta.Type && item.meta.Name == meta.Name {
			s[i] = metaAndTable{meta: meta, table: table}
			return true
		}
	}
	return false
}

func (s storeMeta) find(t timeseries.MetaType, name string) (timeseries.Meta, chx.Table, bool) {
	for _, item := range s {
		if item.meta.Type == t && item.meta.Name == name {
			return item.meta, item.table, true
		}
	}
	return timeseries.Meta{}, chx.Table{}, false
}

func (s storeMeta) listMetaWithAgg() (r []timeseries.Meta) {
	for _, item := range s {
		if item.meta.Aggregation != nil {
			r = append(r, item.meta)
		}
	}
	return r
}

func (s storeMeta) GetNames() []string {
	return utils.MapSliceNoError(s, func(item metaAndTable) string {
		return item.meta.GetFullName()
	})
}

func (s storeMeta) GetTableNames() []string {
	return utils.MapSliceNoError(s, func(item metaAndTable) string {
		return item.table.Name
	})
}

func (s storeMeta) GetHash() string {
	h := sha256.New()
	for _, item := range s {
		h.Write([]byte(item.meta.Hash()))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s storeMeta) GetAllMeta() map[timeseries.MetaType]map[string]timeseries.Meta {
	m := make(map[timeseries.MetaType]map[string]timeseries.Meta)
	for _, item := range s {
		utils.PutIntoK2Map(m, item.meta.Type, item.meta.Name, item.meta)
	}
	return m
}

func (s storeMeta) MetaTypes() []timeseries.MetaType {
	var types []timeseries.MetaType
	for _, item := range s {
		if utils.IndexOf(types, item.meta.Type) < 0 {
			types = append(types, item.meta.Type)
		}
	}
	return types
}

func (s storeMeta) MetaNames(t timeseries.MetaType) []string {
	var names []string
	for _, item := range s {
		if item.meta.Type == t {
			names = append(names, item.meta.Name)
		}
	}
	return names
}

func (s storeMeta) Meta(t timeseries.MetaType, name string) (timeseries.Meta, bool) {
	for _, item := range s {
		if item.meta.Type == t && item.meta.Name == name {
			return item.meta, true
		}
	}
	return timeseries.Meta{}, false
}

func (s storeMeta) MetaByType(t timeseries.MetaType) map[string]timeseries.Meta {
	m := make(map[string]timeseries.Meta)
	for _, item := range s {
		if item.meta.Type == t {
			m[item.meta.Name] = item.meta
		}
	}
	return m
}

func (s storeMeta) MustMeta(t timeseries.MetaType, name string) timeseries.Meta {
	meta, has := s.Meta(t, name)
	if has {
		return meta
	}
	return timeseries.Meta{}
}

func (s storeMeta) Different(other timeseries.StoreMeta) bool {
	return s.GetHash() != other.GetHash()
}

func (s storeMeta) String() string {
	b, _ := sonic.Marshal(s)
	return string(b)
}
