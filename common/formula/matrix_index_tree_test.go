package formula

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatrixIndex(t *testing.T) {
	m := newMatrixIndex()
	m.add(1, "a", "b", "c")
	m.add(2, "a", "b", "d")
	m.add(3, "a", "b", "e")
	require.EqualValues(t, 1, m.get("a", "b", "c"))
	require.EqualValues(t, 2, m.get("a", "b", "d"))
	require.EqualValues(t, 3, m.get("a", "b", "e"))
	require.EqualValues(t, -1, m.get("a", "b", "f"))
}
