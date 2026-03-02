package cte

import "strings"

type CTE struct {
	Alias string
	Query string
}

type CTEs []CTE

func (c CTEs) String() string {
	var cteStr []string
	for _, cte := range c {
		cteStr = append(cteStr, cte.Alias+" AS ("+cte.Query+")")
	}
	if len(cteStr) == 0 {
		return ""
	}
	return "WITH " + strings.Join(cteStr, ", ")
}
