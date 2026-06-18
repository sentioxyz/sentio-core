package aptos

import (
	"fmt"

	"sentioxyz/sentio-core/common/utils"
)

type AccountResourceFilter struct {
	Address string

	// nil means need all resources of the account, empty means do not need any resource
	// only contract move interval handler will use empty ResourceType
	ResourceType []string
}

func (c AccountResourceFilter) NeedNothing() bool {
	return c.ResourceType != nil && len(c.ResourceType) == 0
}

func (c AccountResourceFilter) String() string {
	return fmt.Sprintf("Address:%s,ResourceType:%s", c.Address, utils.ArrSummary(c.ResourceType, 10))
}

func (c AccountResourceFilter) Check(ar AccountResource) bool {
	return c.Address == ar.Address && (c.ResourceType == nil || utils.IndexOf(c.ResourceType, ar.Type) >= 0)
}

func MergeAccountResourceFilters(filters []AccountResourceFilter) map[string][]string {
	result := make(map[string][]string)
	for _, filter := range filters {
		if types, has := result[filter.Address]; !has {
			result[filter.Address] = filter.ResourceType
		} else if types != nil {
			if filter.ResourceType == nil {
				result[filter.Address] = nil
			} else {
				result[filter.Address] = append(types, filter.ResourceType...)
			}
		}
	}
	return result
}

type AccountResource struct {
	Raw string

	Address string
	Type    string
}
