package schema

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
)

type TypeChain []types.Type

func BreakType(typ types.Type) (chain TypeChain) {
	for {
		chain = append(chain, typ)
		switch wrapType := typ.(type) {
		case *types.NonNull:
			typ = wrapType.OfType
		case *types.List:
			typ = wrapType.OfType
		default:
			return
		}
	}
}

func (tc TypeChain) Join() types.Type {
	typ := tc[len(tc)-1]
	for i := len(tc) - 2; i >= 0; i-- {
		switch tc[i].(type) {
		case *types.NonNull:
			typ = &types.NonNull{OfType: typ}
		case *types.List:
			typ = &types.List{OfType: typ}
		}
	}
	return typ
}

func (tc TypeChain) InnerType() types.Type {
	return tc[len(tc)-1]
}

func (tc TypeChain) OuterType() types.Type {
	return tc[0]
}

// InnerTypeNullable
//
//	   String    => true
//		 String!   => false
//		 [String]  => true
//		 [String!] => false
func (tc TypeChain) InnerTypeNullable() bool {
	if len(tc) <= 1 {
		return true
	}
	upper := tc[len(tc)-2]
	_, is := upper.(*types.NonNull)
	return !is
}

func (tc TypeChain) CountListLayer() (count int) {
	for _, typ := range tc {
		if _, is := typ.(*types.List); is {
			count++
		}
	}
	return
}

func (tc TypeChain) SkipListLayer(x int) TypeChain {
	if x == 0 {
		return tc
	}
	for i := 0; i < len(tc); i++ {
		if _, is := tc[i].(*types.List); is {
			x--
			if x == 0 {
				return tc[i+1:]
			}
		}
	}
	return nil
}

func (tc TypeChain) Restructure(newInnerType types.Type) types.Type {
	root := newInnerType
	for i := len(tc) - 2; i >= 0; i-- {
		switch tc[i].(type) {
		case *types.NonNull:
			root = &types.NonNull{OfType: root}
		case *types.List:
			root = &types.List{OfType: root}
		default:
			panic(fmt.Errorf("invalid type chain, unreachable"))
		}
	}
	return root
}
