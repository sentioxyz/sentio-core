package timeseries

import (
	"sentioxyz/sentio-core/common/utils"

	"github.com/pkg/errors"
)

func MergeDatasets(data []Dataset) ([]Dataset, error) {
	dict := make(map[string]Dataset)
	for _, ds := range data {
		fullName := ds.GetFullName()
		if exist, has := dict[fullName]; !has {
			dict[fullName] = ds
		} else if !exist.Aggregation.IsSame(ds.Aggregation) {
			return nil, errors.Wrapf(ErrInvalidMetaDiff, "unacceptable aggregation change for %s", fullName)
		} else if diff := exist.DiffFields(ds.Meta); len(diff.UpdFields) > 0 {
			f := diff.UpdFields[0]
			return nil, errors.Wrapf(ErrInvalidMetaDiff, "types of fields %s.%s are not uniform, including %s and %s",
				fullName, f.Before.Name, f.Before.Type, f.After.Type)
		} else {
			exist.Meta = exist.Merge(ds.Meta)
			exist.Rows = append(exist.Rows, ds.Rows...)
			dict[fullName] = exist
		}
	}
	return utils.GetMapValuesOrderByKey(dict), nil
}
