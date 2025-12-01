package formula

import (
	"testing"

	"sentioxyz/sentio-core/service/common/protos"

	"github.com/stretchr/testify/assert"
)

type testcase struct {
	name       string
	ctx        Context
	expression Expression
	need       Value
	err        error
}

var evaluateTestcases = []testcase{
	{
		"a+b",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
				"b": &ScalarValue{2},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&ScalarValue{3},
		nil,
	},
	{
		"a-b",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
				"b": &ScalarValue{2},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    MINUS,
		},
		&ScalarValue{-1},
		nil,
	},
	{
		"a*b",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
				"b": &ScalarValue{2},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    MUL,
		},
		&ScalarValue{2},
		nil,
	},
	{
		"a/b",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
				"b": &ScalarValue{2},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    DIV,
		},
		&ScalarValue{0.5},
		nil,
	},
	{
		"a/b but b=0",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
				"b": &ScalarValue{0},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    DIV,
		},
		&ScalarValue{0},
		nil,
	},
	{
		"a/b+c",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
				"b": &ScalarValue{2},
				"c": &ScalarValue{5},
			},
		},
		&BinaryExpression{
			Left: &BracketExpression{
				Expr: &BinaryExpression{
					Left:  &Identifier{"a"},
					Right: &Identifier{"b"},
					Op:    DIV,
				},
			},
			Right: &Identifier{"c"},
			Op:    PLUS,
		},
		&ScalarValue{5.5},
		nil,
	},
	{
		"vec(a)+b",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     1,
							},
							{
								Timestamp: 2,
								Value:     3,
							},
							{
								Timestamp: 3,
								Value:     5,
							},
						},
					},
				},
				"b": &ScalarValue{2},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     3,
					},
					{
						Timestamp: 2,
						Value:     5,
					},
					{
						Timestamp: 3,
						Value:     7,
					},
				},
			},
		},
		nil,
	},
	{
		"vec(a)/b",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     10,
							},
							{
								Timestamp: 2,
								Value:     30,
							},
							{
								Timestamp: 3,
								Value:     50,
							},
						},
					},
				},
				"b": &ScalarValue{10},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    DIV,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     1,
					},
					{
						Timestamp: 2,
						Value:     3,
					},
					{
						Timestamp: 3,
						Value:     5,
					},
				},
			},
		},
		nil,
	},
	{
		"vec(a)*b",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     10,
							},
							{
								Timestamp: 2,
								Value:     30,
							},
							{
								Timestamp: 3,
								Value:     50,
							},
						},
					},
				},
				"b": &ScalarValue{10},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    MUL,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     100,
					},
					{
						Timestamp: 2,
						Value:     300,
					},
					{
						Timestamp: 3,
						Value:     500,
					},
				},
			},
		},
		nil,
	},
	{
		"b/vec(a)",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     10,
							},
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     5,
							},
						},
					},
				},
				"b": &ScalarValue{10},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"b"},
			Right: &Identifier{"a"},
			Op:    DIV,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     1,
					},
					{
						Timestamp: 2,
						Value:     5,
					},
					{
						Timestamp: 3,
						Value:     2,
					},
				},
			},
		},
		nil,
	},
	{
		"vec(a)+vec(b)",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     1,
							},
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     3,
							},
						},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     1,
							},
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     3,
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     2,
					},
					{
						Timestamp: 2,
						Value:     4,
					},
					{
						Timestamp: 3,
						Value:     6,
					},
				},
			},
		},
		nil,
	},
	{
		"vec(a)+vec(b) dimension not matched",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     3,
							},
							{
								Timestamp: 4,
								Value:     4,
							},
							{
								Timestamp: 5,
								Value:     5,
							},
							{
								Timestamp: 6,
								Value:     6,
							},
						},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     1,
							},
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     3,
							},
							{
								Timestamp: 4,
								Value:     4,
							},
							{
								Timestamp: 5,
								Value:     5,
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     1,
					},
					{
						Timestamp: 2,
						Value:     4,
					},
					{
						Timestamp: 3,
						Value:     6,
					},
					{
						Timestamp: 4,
						Value:     8,
					},
					{
						Timestamp: 5,
						Value:     10,
					},
					{
						Timestamp: 6,
						Value:     6,
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)+b",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &ScalarValue{10},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20,
						},
						{
							Timestamp: 2,
							Value:     40,
						},
						{
							Timestamp: 3,
							Value:     60,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     110,
						},
						{
							Timestamp: 2,
							Value:     310,
						},
						{
							Timestamp: 3,
							Value:     510,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)*vec(b), dimension(b) is greater than dimension(a)",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     1,
							},
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     3,
							},
							{
								Timestamp: 4,
								Value:     4,
							},
							{
								Timestamp: 5,
								Value:     5,
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    MUL,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     10,
						},
						{
							Timestamp: 2,
							Value:     60,
						},
						{
							Timestamp: 3,
							Value:     150,
						},
						{
							Timestamp: 4,
							Value:     0,
						},
						{
							Timestamp: 5,
							Value:     0,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     600,
						},
						{
							Timestamp: 3,
							Value:     1500,
						},
						{
							Timestamp: 4,
							Value:     0,
						},
						{
							Timestamp: 5,
							Value:     0,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)*matrix(b)",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    MUL,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     900,
						},
						{
							Timestamp: 3,
							Value:     2500,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     10000,
						},
						{
							Timestamp: 2,
							Value:     90000,
						},
						{
							Timestamp: 3,
							Value:     250000,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)+matrix(b), label not matched",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "india",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20,
						},
						{
							Timestamp: 2,
							Value:     60,
						},
						{
							Timestamp: 3,
							Value:     100,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "india",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     300,
						},
						{
							Timestamp: 3,
							Value:     500,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     300,
						},
						{
							Timestamp: 3,
							Value:     500,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"b-matrix(a)",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &ScalarValue{100},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"b"},
			Right: &Identifier{"a"},
			Op:    MINUS,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     90,
						},
						{
							Timestamp: 2,
							Value:     70,
						},
						{
							Timestamp: 3,
							Value:     50,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     0,
						},
						{
							Timestamp: 2,
							Value:     -200,
						},
						{
							Timestamp: 3,
							Value:     -400,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"vec(b)/matrix(a)",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     100,
							},
							{
								Timestamp: 2,
								Value:     300,
							},
							{
								Timestamp: 3,
								Value:     500,
							},
							{
								Timestamp: 4,
								Value:     1000,
							},
							{
								Timestamp: 5,
								Value:     1500,
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"b"},
			Right: &Identifier{"a"},
			Op:    DIV,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     10,
						},
						{
							Timestamp: 2,
							Value:     10,
						},
						{
							Timestamp: 3,
							Value:     10,
						},
						{
							Timestamp: 4,
							Value:     0,
						},
						{
							Timestamp: 5,
							Value:     0,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     1,
						},
						{
							Timestamp: 2,
							Value:     1,
						},
						{
							Timestamp: 3,
							Value:     1,
						},
						{
							Timestamp: 4,
							Value:     0,
						},
						{
							Timestamp: 5,
							Value:     0,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"a",
		Context{
			Values: map[string]Value{
				"a": &ScalarValue{1},
			},
		},
		&AggregateExpression{
			Expr: &Identifier{"a"},
			Op:   MAX,
		},
		&ScalarValue{1},
		nil,
	},
	{
		"vec(b) sum",
		Context{
			Values: map[string]Value{
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     100,
							},
							{
								Timestamp: 2,
								Value:     300,
							},
							{
								Timestamp: 3,
								Value:     500,
							},
							{
								Timestamp: 4,
								Value:     1000,
							},
							{
								Timestamp: 5,
								Value:     1500,
							},
						},
					},
				},
			},
		},
		&AggregateExpression{
			Expr: &Identifier{"b"},
			Op:   SUM,
		},
		&ScalarValue{Value: 3400},
		nil,
	},
	{
		"matrix(a)*max(vec(b)), dimension(b) is greater than dimension(a)",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     1,
							},
							{
								Timestamp: 2,
								Value:     2,
							},
							{
								Timestamp: 3,
								Value:     3,
							},
							{
								Timestamp: 4,
								Value:     4,
							},
							{
								Timestamp: 5,
								Value:     5,
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left: &Identifier{"a"},
			Right: &AggregateExpression{
				Expr: &Identifier{"b"},
				Op:   MAX,
			},
			Op: MUL,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     50,
						},
						{
							Timestamp: 2,
							Value:     150,
						},
						{
							Timestamp: 3,
							Value:     250,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     500,
						},
						{
							Timestamp: 2,
							Value:     1500,
						},
						{
							Timestamp: 3,
							Value:     2500,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"min(matrix(a))",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
								{
									Timestamp: 4,
									Value:     -100,
								},
							},
						},
					},
				},
			},
		},
		&AggregateExpression{
			Expr: &Identifier{"a"},
			Op:   MIN,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "",
					DisplayName: "",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     10,
					},
					{
						Timestamp: 2,
						Value:     30,
					},
					{
						Timestamp: 3,
						Value:     50,
					},
					{
						Timestamp: 4,
						Value:     -100,
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)*matrix(b),label is not same",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
									"coin":    "btc",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
									"coin":    "btc",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
									"coin":    "eth",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     20,
								},
								{
									Timestamp: 2,
									Value:     40,
								},
								{
									Timestamp: 3,
									Value:     60,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
									"coin":    "eth",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     200,
								},
								{
									Timestamp: 2,
									Value:     400,
								},
								{
									Timestamp: 3,
									Value:     600,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    MUL,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
							"coin":    "btc",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     900,
						},
						{
							Timestamp: 3,
							Value:     2500,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
							"coin":    "btc",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     10000,
						},
						{
							Timestamp: 2,
							Value:     90000,
						},
						{
							Timestamp: 3,
							Value:     250000,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
							"coin":    "eth",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     200,
						},
						{
							Timestamp: 2,
							Value:     1200,
						},
						{
							Timestamp: 3,
							Value:     3000,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
							"coin":    "eth",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20000,
						},
						{
							Timestamp: 2,
							Value:     120000,
						},
						{
							Timestamp: 3,
							Value:     300000,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)*matrix(b),label is not same",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
									"coin":    "btc",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
									"coin":    "btc",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
									"coin":    "eth",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     20,
								},
								{
									Timestamp: 2,
									Value:     40,
								},
								{
									Timestamp: 3,
									Value:     60,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
									"coin":    "eth",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     200,
								},
								{
									Timestamp: 2,
									Value:     400,
								},
								{
									Timestamp: 3,
									Value:     600,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"b"},
			Right: &Identifier{"a"},
			Op:    MUL,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
							"coin":    "btc",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     900,
						},
						{
							Timestamp: 3,
							Value:     2500,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
							"coin":    "btc",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     10000,
						},
						{
							Timestamp: 2,
							Value:     90000,
						},
						{
							Timestamp: 3,
							Value:     250000,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
							"coin":    "eth",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     200,
						},
						{
							Timestamp: 2,
							Value:     1200,
						},
						{
							Timestamp: 3,
							Value:     3000,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
							"coin":    "eth",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20000,
						},
						{
							Timestamp: 2,
							Value:     120000,
						},
						{
							Timestamp: 3,
							Value:     300000,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)*matrix(b),label not support",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"coin": "btc",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"coin": "usdt",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"b"},
			Right: &Identifier{"a"},
			Op:    MUL,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"coin": "btc",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     0,
						},
						{
							Timestamp: 2,
							Value:     0,
						},
						{
							Timestamp: 3,
							Value:     0,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"coin": "usdt",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     0,
						},
						{
							Timestamp: 2,
							Value:     0,
						},
						{
							Timestamp: 3,
							Value:     0,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"empty vector",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     3,
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "b",
					DisplayName: "d_b",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     3,
					},
				},
			},
		},
		nil,
	},
	{
		"empty vector",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{},
					},
				},
				"b": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "b",
							DisplayName: "d_b",
						},
						Values: []*protos.Matrix_Value{},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "b",
					DisplayName: "d_b",
				},
				Values: []*protos.Matrix_Value{},
			},
		},
		nil,
	},
	{
		"matrix(a)+matrix(b), length not match #1",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "india",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     1000,
								},
								{
									Timestamp: 2,
									Value:     3000,
								},
								{
									Timestamp: 3,
									Value:     5000,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"a"},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20,
						},
						{
							Timestamp: 2,
							Value:     60,
						},
						{
							Timestamp: 3,
							Value:     100,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "india",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     300,
						},
						{
							Timestamp: 3,
							Value:     500,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     1000,
						},
						{
							Timestamp: 2,
							Value:     3000,
						},
						{
							Timestamp: 3,
							Value:     5000,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"matrix(a)+matrix(b), length not match #2",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "india",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     1000,
								},
								{
									Timestamp: 2,
									Value:     3000,
								},
								{
									Timestamp: 3,
									Value:     5000,
								},
							},
						},
					},
				},
				"b": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     10,
								},
								{
									Timestamp: 2,
									Value:     30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
					},
				},
			},
		},
		&BinaryExpression{
			Left:  &Identifier{"b"},
			Right: &Identifier{"a"},
			Op:    PLUS,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20,
						},
						{
							Timestamp: 2,
							Value:     60,
						},
						{
							Timestamp: 3,
							Value:     100,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "india",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     100,
						},
						{
							Timestamp: 2,
							Value:     300,
						},
						{
							Timestamp: 3,
							Value:     500,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     1000,
						},
						{
							Timestamp: 2,
							Value:     3000,
						},
						{
							Timestamp: 3,
							Value:     5000,
						},
					},
				},
			},
		},
		nil,
	},
	{
		"abs(a)+abs(b)",
		Context{
			Values: map[string]Value{
				"a": &VectorValue{
					sample: &protos.Matrix_Sample{
						Metric: &protos.Matrix_Metric{
							Labels:      map[string]string{},
							Name:        "a",
							DisplayName: "d_a",
						},
						Values: []*protos.Matrix_Value{
							{
								Timestamp: 1,
								Value:     -1,
							},
							{
								Timestamp: 2,
								Value:     3,
							},
							{
								Timestamp: 3,
								Value:     -5,
							},
						},
					},
				},
				"b": &ScalarValue{-2},
			},
		},
		&BinaryExpression{
			Left:  &AggregateExpression{&Identifier{"a"}, ABS},
			Right: &AggregateExpression{&Identifier{"b"}, ABS},
			Op:    PLUS,
		},
		&VectorValue{
			sample: &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels:      map[string]string{},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{
					{
						Timestamp: 1,
						Value:     3,
					},
					{
						Timestamp: 2,
						Value:     5,
					},
					{
						Timestamp: 3,
						Value:     7,
					},
				},
			},
		},
		nil,
	},
	{
		"abs(matrix(a))+b",
		Context{
			Values: map[string]Value{
				"a": &MatrixValue{
					samples: []*protos.Matrix_Sample{
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "china",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     -10,
								},
								{
									Timestamp: 2,
									Value:     -30,
								},
								{
									Timestamp: 3,
									Value:     50,
								},
							},
						},
						{
							Metric: &protos.Matrix_Metric{
								Labels: map[string]string{
									"country": "usa",
								},
								Name:        "a",
								DisplayName: "d_a",
							},
							Values: []*protos.Matrix_Value{
								{
									Timestamp: 1,
									Value:     100,
								},
								{
									Timestamp: 2,
									Value:     -300,
								},
								{
									Timestamp: 3,
									Value:     500,
								},
							},
						},
					},
				},
				"b": &ScalarValue{10},
			},
		},
		&BinaryExpression{
			Left:  &AggregateExpression{&Identifier{"a"}, ABS},
			Right: &Identifier{"b"},
			Op:    PLUS,
		},
		&MatrixValue{
			samples: []*protos.Matrix_Sample{
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "china",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     20,
						},
						{
							Timestamp: 2,
							Value:     40,
						},
						{
							Timestamp: 3,
							Value:     60,
						},
					},
				},
				{
					Metric: &protos.Matrix_Metric{
						Labels: map[string]string{
							"country": "usa",
						},
						Name:        "a",
						DisplayName: "d_a",
					},
					Values: []*protos.Matrix_Value{
						{
							Timestamp: 1,
							Value:     110,
						},
						{
							Timestamp: 2,
							Value:     310,
						},
						{
							Timestamp: 3,
							Value:     510,
						},
					},
				},
			},
		},
		nil,
	},
}

func TestEvaluate(t *testing.T) {
	for _, testcase := range evaluateTestcases {
		t.Run(testcase.name, func(t *testing.T) {
			value, err := Evaluate(testcase.ctx, testcase.expression)
			if testcase.err == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if testcase.err == nil && !value.equal(testcase.need) {
				t.Errorf("expected %s, got %s", testcase.need.CheckSum(), value.CheckSum())
				t.Log(string(value.Debug()))
			}
		})
	}
}

func TestMatrixValue_Labels(t *testing.T) {
	m := &MatrixValue{
		samples: []*protos.Matrix_Sample{
			{
				Metric: &protos.Matrix_Metric{
					Labels: map[string]string{
						"country":     "china",
						"coin_symbol": "cny",
					},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{},
			},
			{
				Metric: &protos.Matrix_Metric{
					Labels: map[string]string{
						"country":     "india",
						"coin_symbol": "inr",
					},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{},
			},
			{
				Metric: &protos.Matrix_Metric{
					Labels: map[string]string{
						"country": "usa",
						"amount":  "100",
					},
					Name:        "a",
					DisplayName: "d_a",
				},
				Values: []*protos.Matrix_Value{},
			},
		},
	}
	labels := m.Labels()
	assert.EqualValues(t, 3, len(labels))
	index := m.buildIndex()
	assert.EqualValues(t, 0, index.get("amount=", "coin_symbol=cny", "country=china"))
	assert.EqualValues(t, 1, index.get("amount=", "coin_symbol=inr", "country=india"))
	assert.EqualValues(t, 2, index.get("amount=100", "coin_symbol=", "country=usa"))
}
