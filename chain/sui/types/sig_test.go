package types

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeMultiSig(t *testing.T) {
	s := `AwIAvlJnUP0iJFZL+QTxkKC9FHZGwCa5I4TITHS/QDQ12q1sYW6SMt2Yp3PSNzsAay0Fp2MPVohqyyA02UtdQ2RNAQGH0eLk4ifl9h1I8Uc+4QlRYfJC21dUbP8aFaaRqiM/f32TKKg/4PSsGf9lFTGwKsHJYIMkDoqKwI8Xqr+3apQzAwADAFriILSy9l6XfBLt5hV5/1FwtsIsAGFow3tefGGvAYCDAQECHRUjB8a3Kw7QQYsOcM2A5/UpW42G9XItP1IT+9I5TzYCADtqJ7zOtqQtYqOo0CpvDXNlMhV3HeJDpjrASKGLWdopAwMA`

	sig, err := DecodeMultiSig(s)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(sig.Sigs))
	assert.Equal(t, 3, len(sig.PublicKey.PkMap))
	fmt.Println(sig.DebugString())

	s = `AwkAmdgYPD5NxEk1ysG+PJYKGnKPq67qcxMmLeYU6AA/8Te8qZkyYkaFFCX5BMDgxCx2IptmUe3e9+T10CqDw1niCACbkrC1vCR9sQbX7wGLG8hi6c0UuNmRN2IXoz3iSfcobNlb2VYjQ6kgV+b+Cwfjv0T0eO5qapywM574Qpbp1r4GAOEFjW+oqJR+wHockpQBcbDwBfhTMVETsq6z7nIIlSHny8ETvpI2eZ0AGDCeojCQgNzUh0jnSX0WFFZKWTfDcgsAt1BlPbI2c46A34azqCHg9TfGA/8W86Jn8TF1UV3t2mY5w8lAie2R4VXcZXIjdefZTTxapKpz3PH0nT1MT77XCQBQPOEI7ZnmBQkm6QdunGvk8twynEUs1LuGQUfKA//m8jUNFI08C9Zwyhrj8Y6zeqlDu9O3TE40yiq6Q5BBCjAFAKLJUm6OatjPmLOBE9Bet6/q8cb//PzBJ2qyq/UBMMH5Qh06solYoz4c3q1v4vXOxtsRpFfE4kIF6rk3T4evHA0AR+Lm7es62VKZFZL6UP06IFstkoWgx1eQ8y8PWTrEzc0zrbUAYWvvgarafWakzV0fvJwmrs6mWHVaWPpxfR5UAwCUGJo0Ejnrm+aTsWWTElRABcWbngrB74bF+JGlkuXc3zmi9TBolmW/mc84u1YDAa4nMsMNYSM0AYa/GISNdL8GABytmGrBdOyWy/lccagO0itwlJxMDSzL0DK2xsfXDtyKdat46gqmqng3vaUG+OsmVN4SQeIALBuJkzPphus/Egz/AQoAZ4Aer2KARjhqewIPjWagCs10oCmqfc23o63oIonUwAIBAI0+wTCqcqYF5WE1KQp+VUolZ4qZ5iPHBlFAC/+melmxAQBGL5lmI8v9u2lhNdccF2MjzvEbptVCLHMzE80w8Jg6ZwEAHqYnrOvIUDEFCHZTT6awiXK7xx59cIMF5ZhAEJy3cGUBAOLzHGux38QDb6wRMIZG6XPqZMTKZPA2OdtIhEhhJIpKAQDX+ze6+kW2Tx9yI+psNRnBl7nCIKNgpg1od7MaeyV0zQEA39L7fKsYIT662KMBlCQ1PLsoeap0eNKtggmOVP8h1CwBAFfhHd1PUD32EiacpT7ZZDdApkqfRI18IYLf9T2GlBqlAQD83lfOxft/LoWJAZobBm6n1jBgbiyjAxdb3YMlUvFP8AEAbWF2ZW4wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABCQA=`
	sig, err = DecodeMultiSig(s)
	assert.NoError(t, err)
	found := false
	for _, pk := range sig.PublicKey.PkMap {
		k := pk.PubKey.ED25519
		if bytes.HasPrefix(k[:], []byte("maven0")) {
			fmt.Println("found maven0", hex.EncodeToString(k[:]))
			found = true
		}
	}
	assert.True(t, found)
}
