package sui

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"testing"
)

func Test_ObjectChangeFilter(t *testing.T) {
	f := ObjectChangeFilter{
		TypePattern: move.TypeSet{move.MustBuildType("0x1::"), move.MustBuildType("0x1111::")},
		OwnerFilter: &ObjectChangeOwnerFilter{OwnerID: []string{"0x2", "0x2222"}},
		ObjectIDIn:  set.New("aa"),
	}
	b, err := json.Marshal(f)
	assert.NoError(t, err)
	assert.Equal(t, `{"type_pattern":["0x1::*::*","0x1111::*::*"],"owner_filter":{"owner_id":["0x2","0x2222"]},"object_id_in":["aa"]}`, string(b))

	var r ObjectChangeFilter
	assert.NoError(t, json.Unmarshal(b, &r))
	assert.Equal(t, f, r)

	assert.NoError(t, json.Unmarshal([]byte(`{"object_id_in":["bb",""]}`), &f))
	assert.Equal(t, 2, f.ObjectIDIn.Size())
	assert.True(t, f.ObjectIDIn.Contains("bb"))
	assert.True(t, f.ObjectIDIn.Contains(""))
	assert.Nil(t, f.TypePattern)
	assert.Nil(t, f.OwnerFilter)

	assert.NoError(t, json.Unmarshal([]byte(`{}`), &f))
	assert.Equal(t, 0, f.ObjectIDIn.Size())
	assert.Nil(t, f.TypePattern)
	assert.Nil(t, f.OwnerFilter)

	f1 := ObjectChangeFilter{
		TypePattern: move.TypeSet{move.MustBuildType("0x1::"), move.MustBuildType("0x1111::")},
		OwnerFilter: &ObjectChangeOwnerFilter{OwnerID: []string{"0x2", "0x2222"}},
	}
	b, err = json.Marshal(f1)
	assert.NoError(t, err)
	assert.Equal(t, `{"type_pattern":["0x1::*::*","0x1111::*::*"],"owner_filter":{"owner_id":["0x2","0x2222"]}}`, string(b))

	f2 := ObjectChangeFilter{
		TypePattern: move.TypeSet{move.MustBuildType("0x1::"), move.MustBuildType("0x1111::")},
		OwnerFilter: &ObjectChangeOwnerFilter{OwnerID: []string{"0x2", "0x2222"}},
		ObjectIDIn:  set.New("aa"),
	}
	b, err = json.Marshal(f1.Merge(f2))
	assert.NoError(t, err)
	assert.Equal(t, `{"type_pattern":["0x1::*::*","0x1111::*::*"],"owner_filter":{"owner_id":["0x2","0x2222"]}}`, string(b))
}

func Test_panicThenPanic(t *testing.T) {
	fn := func() {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				if err, is := panicErr.(error); is {
					panic(errors.Wrapf(err, "level2"))
				}
				panic(errors.Errorf("level2: %v", panicErr))
			}
		}()
		panic(errors.Errorf("level1"))
	}
	fn2 := func() (err error) {
		defer func() {
			if pe := recover(); pe != nil {
				var is bool
				if err, is = pe.(error); !is {
					err = errors.Errorf("got panic: %v", pe)
				}
			}
		}()
		fn()
		return nil
	}
	err := fn2()
	assert.NotNil(t, err)
	assert.Equal(t, "level2: level1", err.Error())
	log.Errorfe(err, "final error")
}
