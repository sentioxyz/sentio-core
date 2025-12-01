package anyutil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_has(t *testing.T) {
	orig := `
{
	"k1": "v1",
	"k2": null
}
`
	var obj any
	assert.NoError(t, json.Unmarshal([]byte(orig), &obj))

	v, has := GetPropertyByPath(obj, "k2")
	assert.Equal(t, nil, v)
	assert.Equal(t, true, has)
}

func Test_getter(t *testing.T) {
	obj := map[string]any{
		"p1":  "v1",
		"p-2": "v2",
		"child1": map[any]any{
			"p3": "v3",
			"p4": "v4",
			"child11": map[string]any{
				"p5": "v5",
			},
		},
		"child2": []string{"v5", "v6"},
		"child3": []any{
			"v7",
			123,
			map[string]any{
				"p6": "v6",
				"p7": "v7",
			},
		},
		"child4": []any{
			map[string]any{
				"p8": map[string]any{
					"p10": "v10",
				},
			},
			map[string]any{
				"p8": "v82",
				"p9": "v92",
			},
			map[string]any{
				"p9": "v93",
			},
		},
	}

	assert.Equal(t, "v1", GetPropertyByPathWithDefault(obj, "p1", nil))
	assert.Equal(t, "v2", GetPropertyByPathWithDefault(obj, "p-2", nil))
	assert.Equal(t, nil, GetPropertyByPathWithDefault(obj, "p3", nil))
	assert.Equal(t, "v3", GetPropertyByPathWithDefault(obj, "child1.p3", nil))
	assert.Equal(t, "v4", GetPropertyByPathWithDefault(obj, "child1.p4", nil))
	assert.Equal(t, nil, GetPropertyByPathWithDefault(obj, "child1.p5", nil))
	assert.Equal(t, "v5", GetPropertyByPathWithDefault(obj, "child1.child11.p5", nil))
	assert.Equal(t, nil, GetPropertyByPathWithDefault(obj, "child1.child11.p6", nil))
	assert.Equal(t, "v5", GetPropertyByPathWithDefault(obj, "child2[0]", nil))
	assert.Equal(t, "v6", GetPropertyByPathWithDefault(obj, "child2[1]", nil))
	assert.Equal(t, nil, GetPropertyByPathWithDefault(obj, "child2[2]", nil))
	assert.Equal(t, "v6", GetPropertyByPathWithDefault(obj, "child2[-1]", nil))
	assert.Equal(t, "v5", GetPropertyByPathWithDefault(obj, "child2[-2]", nil))
	assert.Equal(t, nil, GetPropertyByPathWithDefault(obj, "child2[-3]", nil))
	assert.Equal(t, "v7", GetPropertyByPathWithDefault(obj, "child3[0]", nil))
	assert.Equal(t, 123, GetPropertyByPathWithDefault(obj, "child3[1]", nil))
	assert.Equal(t, "v6", GetPropertyByPathWithDefault(obj, "child3[2].p6", nil))
	assert.Equal(t, "v7", GetPropertyByPathWithDefault(obj, "child3[2].p7", nil))
	assert.Equal(t, "v7", GetPropertyByPathWithDefault(obj, "child3[-1].p7", nil))
	assert.Equal(t, nil, GetPropertyByPathWithDefault(obj, "child3[2].p8", nil))
	assert.Equal(t, []any{"v5", "v6"}, GetPropertyByPathWithDefault(obj, "child2[*]", nil))
	assert.Equal(t, []any{nil, "v92", "v93"}, GetPropertyByPathWithDefault(obj, "child4[*].p9", nil))
	assert.Equal(t, []any{"v10", nil, nil}, GetPropertyByPathWithDefault(obj, "child4[*].p8.p10", nil))
}
