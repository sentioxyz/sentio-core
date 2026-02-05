package exp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_newExpFailed1(t *testing.T) {
	cases := []string{
		"",
		" ",
		"\t",
		" \t",
		"\n",
		"\r",
		" \n",
	}
	for i, exp := range cases {
		_, err := NewExp(exp)
		assert.EqualErrorf(t, err, "empty expression", "case #%d: %s", i, exp)
	}
}

func Test_newExpFailed2(t *testing.T) {
	cases := []string{
		"(",
		"(v1",
		"(1",
		"(1.1",
		"(1)+(2",
		"((v1+1)*(v2+2)",
	}
	for i, exp := range cases {
		_, err := NewExp(exp)
		assert.EqualErrorf(t, err, "miss ')' at the end of the exp", "case #%d: %s", i, exp)
	}
}

func Test_newExpFailed3(t *testing.T) {
	cases := []string{
		")",
		"v1)",
		"1)",
		"1.1)",
		"(1))+(2)",
		")(v1+1)",
		"(v1+1))+(v2+2",
	}
	for i, exp := range cases {
		_, err := NewExp(exp)
		assert.ErrorContainsf(t, err, "miss '(' for expression", "case #%d: %s", i, exp)
	}
}

func Test_newExpFailed4(t *testing.T) {
	cases := []string{
		"(&",
		"(%)",
		"(v1@1)",
		"min(a,b,^)",
	}
	for i, exp := range cases {
		_, err := NewExp(exp)
		assert.ErrorContainsf(t, err, "invalid character '", "case #%d: %s", i, exp)
	}
}

func Test_newExpFailed5(t *testing.T) {
	cases := []string{
		"()",
		"(())",
		"((( )))",
	}
	for i, exp := range cases {
		_, err := NewExp(exp)
		assert.ErrorContainsf(t, err, "missing content before ')' at expression[", "case #%d: %s", i, exp)
	}
}

func Test_newExpFailed6(t *testing.T) {
	var err error
	_, err = NewExp("a,b")
	assert.EqualError(t, err, "unexpected ',' at expression[1]")
	_, err = NewExp("a+b,c")
	assert.EqualError(t, err, "unexpected ',' at expression[3]")
	_, err = NewExp("min(a,b,)")
	assert.EqualError(t, err, "missing content before ')' at expression[8]")
	_, err = NewExp("min(a,,)")
	assert.EqualError(t, err, "missing content before ',' at expression[6]")
}

func Test_newExpFailed7(t *testing.T) {
	var err error
	_, err = NewExp("a b")
	assert.EqualError(t, err, "unexpected 'b' at expression[2], the operator may be missing")
	_, err = NewExp("min(a,b)cc")
	assert.EqualError(t, err, "unexpected 'cc' at expression[8..9], the operator may be missing")
	_, err = NewExp("min(a,b) (cc)")
	assert.EqualError(t, err, "unexpected '(' at expression[9], the operator may be missing")
	_, err = NewExp("max min (a,b) cc")
	assert.EqualError(t, err, "unexpected 'min' at expression[4..6], the operator may be missing")
	_, err = NewExp("a + b c")
	assert.EqualError(t, err, "unexpected 'c' at expression[6], the operator may be missing")
}

func Test_newExpSuccess1(t *testing.T) {
	var e *Exp
	var err error

	e, err = NewExp("v1")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Value: &Word{
			Cnt:      "v1",
			Position: Position{S: 0, E: 1},
		},
	}, e)

	e, err = NewExp("1")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Value: &Word{
			Cnt:      "1",
			Position: Position{S: 0, E: 0},
		},
	}, e)

	e, err = NewExp("v1+1")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 2, E: 2},
		},
		Arguments: []*Exp{{
			Value: &Word{
				Cnt:      "v1",
				Position: Position{S: 0, E: 1},
			},
		}, {
			Value: &Word{
				Cnt:      "1",
				Position: Position{S: 3, E: 3},
			},
		}},
	}, e)

	e, err = NewExp("v1+v2+1")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 5, E: 5},
		},
		Arguments: []*Exp{{
			Operator: &Word{
				Cnt:      "+",
				Position: Position{S: 2, E: 2},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v1",
					Position: Position{S: 0, E: 1},
				},
			}, {
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 3, E: 4},
				},
			}},
		}, {
			Value: &Word{
				Cnt:      "1",
				Position: Position{S: 6, E: 6},
			},
		}},
	}, e)

	e, err = NewExp("v1*v2+1")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 5, E: 5},
		},
		Arguments: []*Exp{{
			Operator: &Word{
				Cnt:      "*",
				Position: Position{S: 2, E: 2},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v1",
					Position: Position{S: 0, E: 1},
				},
			}, {
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 3, E: 4},
				},
			}},
		}, {
			Value: &Word{
				Cnt:      "1",
				Position: Position{S: 6, E: 6},
			},
		}},
	}, e)

	e, err = NewExp("v1+(v2+1)")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 2, E: 2},
		},
		Arguments: []*Exp{{
			Value: &Word{
				Cnt:      "v1",
				Position: Position{S: 0, E: 1},
			},
		}, {
			Operator: &Word{
				Cnt:      "+",
				Position: Position{S: 6, E: 6, Lvl: 1},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 4, E: 5, Lvl: 1},
				},
			}, {
				Value: &Word{
					Cnt:      "1",
					Position: Position{S: 7, E: 7, Lvl: 1},
				},
			}},
		}},
	}, e)

	e, err = NewExp("v1+ v2*2")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 2, E: 2},
		},
		Arguments: []*Exp{{
			Value: &Word{
				Cnt:      "v1",
				Position: Position{S: 0, E: 1},
			},
		}, {
			Operator: &Word{
				Cnt:      "*",
				Position: Position{S: 6, E: 6},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 4, E: 5},
				},
			}, {
				Value: &Word{
					Cnt:      "2",
					Position: Position{S: 7, E: 7},
				},
			}},
		}},
	}, e)

	e, err = NewExp("v1+ v2 and 1")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 2, E: 2},
		},
		Arguments: []*Exp{{
			Value: &Word{
				Cnt:      "v1",
				Position: Position{S: 0, E: 1},
			},
		}, {
			Operator: &Word{
				Cnt:      "and",
				Position: Position{S: 7, E: 9},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 4, E: 5},
				},
			}, {
				Value: &Word{
					Cnt:      "1",
					Position: Position{S: 11, E: 11},
				},
			}},
		}},
	}, e)

	e, err = NewExp("v1 or v2 and v3")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "or",
			Position: Position{S: 3, E: 4},
		},
		Arguments: []*Exp{{
			Value: &Word{
				Cnt:      "v1",
				Position: Position{S: 0, E: 1},
			},
		}, {
			Operator: &Word{
				Cnt:      "and",
				Position: Position{S: 9, E: 11},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 6, E: 7},
				},
			}, {
				Value: &Word{
					Cnt:      "v3",
					Position: Position{S: 13, E: 14},
				},
			}},
		}},
	}, e)

	e, err = NewExp("v0 + v1 + max(((v2 * v3)), v4)")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "+",
			Position: Position{S: 8, E: 8},
		},
		Arguments: []*Exp{{
			Operator: &Word{
				Cnt:      "+",
				Position: Position{S: 3, E: 3},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v0",
					Position: Position{S: 0, E: 1},
				},
			}, {
				Value: &Word{
					Cnt:      "v1",
					Position: Position{S: 5, E: 6},
				},
			}},
		}, {
			Operator: &Word{
				Cnt:      "max",
				Position: Position{S: 10, E: 12},
			},
			Arguments: []*Exp{{
				Operator: &Word{
					Cnt:      "*",
					Position: Position{S: 19, E: 19, Lvl: 3},
				},
				Arguments: []*Exp{{
					Value: &Word{
						Cnt:      "v2",
						Position: Position{S: 16, E: 17, Lvl: 3},
					},
				}, {
					Value: &Word{
						Cnt:      "v3",
						Position: Position{S: 21, E: 22, Lvl: 3},
					},
				}},
			}, {
				Value: &Word{
					Cnt:      "v4",
					Position: Position{S: 27, E: 28, Lvl: 1},
				},
			}},
		}},
	}, e)
}

func Test_newExpSuccess2(t *testing.T) {
	var e *Exp
	var err error

	e, err = NewExp("(v1)")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Value: &Word{
			Cnt:      "v1",
			Position: Position{S: 1, E: 2, Lvl: 1},
		},
	}, e)

	e, err = NewExp("((v1))")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Value: &Word{
			Cnt:      "v1",
			Position: Position{S: 2, E: 3, Lvl: 2},
		},
	}, e)

	e, err = NewExp("(((v1)))")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Value: &Word{
			Cnt:      "v1",
			Position: Position{S: 3, E: 4, Lvl: 3},
		},
	}, e)

	e, err = NewExp("((v1)) * (v2 + 1.234)")
	assert.NoError(t, err)
	assert.Equal(t, &Exp{
		Operator: &Word{
			Cnt:      "*",
			Position: Position{S: 7, E: 7},
		},
		Arguments: []*Exp{{
			Value: &Word{
				Cnt:      "v1",
				Position: Position{S: 2, E: 3, Lvl: 2},
			},
		}, {
			Operator: &Word{
				Cnt:      "+",
				Position: Position{S: 13, E: 13, Lvl: 1},
			},
			Arguments: []*Exp{{
				Value: &Word{
					Cnt:      "v2",
					Position: Position{S: 10, E: 11, Lvl: 1},
				},
			}, {
				Value: &Word{
					Cnt:      "1.234",
					Position: Position{S: 15, E: 19, Lvl: 1},
				},
			}},
		}},
	}, e)
}

func Test_expToString(t *testing.T) {
	testcases := [][2]string{
		{"v1", "v1"},
		{"(v1)", "v1"},
		{"((v1))", "v1"},
		{"((v1+1))", "v1 + 1"},
		{"v1+1+2", "(v1 + 1) + 2"},
		{"(v1+1)+2", "(v1 + 1) + 2"},
		{"((v1+1))+2", "(v1 + 1) + 2"},
		{"v1+(1+2)", "v1 + (1 + 2)"},
		{"v1+((1+2))", "v1 + (1 + 2)"},
		{"v1+1*2", "v1 + (1 * 2)"},
		{"v1+(1*2)", "v1 + (1 * 2)"},
		{"(v1+1)*2", "(v1 + 1) * 2"},
		{"(v1+1)*(2+v3)", "(v1 + 1) * (2 + v3)"},
		{"(v1+1)*2*v3", "((v1 + 1) * 2) * v3"},
		{"(v1+1)*(2*v3)", "(v1 + 1) * (2 * v3)"},
		{"v1*1+2*v3", "(v1 * 1) + (2 * v3)"},
		{"(v1*1)+(2*v3)", "(v1 * 1) + (2 * v3)"},
		{"v1*(1+2)*v3", "(v1 * (1 + 2)) * v3"},
		{"v1*((1+2)*v3)", "v1 * ((1 + 2) * v3)"},
		{"v1*((1+min(2,3))*v3)", "v1 * ((1 + min_test(2, 3)) * v3)"},
		{"v1*((max(2,3)+1)*v3)", "v1 * ((max(2, 3) + 1) * v3)"},
	}
	for i, testcase := range testcases {
		e, err := NewExp(testcase[0])
		assert.NoError(t, err)
		assert.Equalf(t, testcase[1], e.Text(testAliasController{}), "testcase #%d: %v", i, testcase)
	}
}

type testAliasController struct {
	EmptyAliasController
}

func (c testAliasController) GetOpName(org string) string {
	switch org {
	case "min":
		return "min_test"
	default:
		return org
	}
}
