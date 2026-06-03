package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rg "sentioxyz/sentio-core/common/range"
)

type fakeRangeStore struct {
	r rg.Range
}

func (f fakeRangeStore) Get(context.Context) (rg.Range, error) { return f.r, nil }
func (f fakeRangeStore) Update(context.Context, rg.RangeOperator) (rg.Range, error) {
	return f.r, nil
}

// recorder returns a loader that records the range it was called with and yields a single tagged
// element.
func recorder(tag string, calls *[]rg.Range) func(context.Context, rg.Range) ([]string, error) {
	return func(_ context.Context, qr rg.Range) ([]string, error) {
		*calls = append(*calls, qr)
		return []string{tag}, nil
	}
}

func TestCheckRangeWithFallback_ExceedsUpperBound(t *testing.T) {
	// ClickHouse covers [100, 200]; a query up to 250 is not synced yet => error (so caller retries).
	store := fakeRangeStore{r: rg.NewRange(100, 200)}
	var ch, bq []rg.Range
	loader := CheckRangeWithFallback[string](store, recorder("ch", &ch), recorder("bq", &bq))

	_, err := loader(context.Background(), rg.NewRange(150, 250))
	require.Error(t, err)
	assert.Empty(t, ch, "primary must not be called when the range exceeds the upper bound")
	assert.Empty(t, bq, "fallback must not be called when the range exceeds the upper bound")
}

func TestCheckRangeWithFallback_WithinPrimary(t *testing.T) {
	store := fakeRangeStore{r: rg.NewRange(100, 200)}
	var ch, bq []rg.Range
	loader := CheckRangeWithFallback[string](store, recorder("ch", &ch), recorder("bq", &bq))

	res, err := loader(context.Background(), rg.NewRange(120, 180))
	require.NoError(t, err)
	assert.Equal(t, []string{"ch"}, res)
	require.Len(t, ch, 1)
	assert.Equal(t, rg.NewRange(120, 180), ch[0])
	assert.Empty(t, bq)
}

func TestCheckRangeWithFallback_StraddlesLowerBound(t *testing.T) {
	// Query [50, 150] straddles the ClickHouse lower bound 100: ClickHouse serves [100,150],
	// BigQuery serves the older [50, 99].
	store := fakeRangeStore{r: rg.NewRange(100, 200)}
	var ch, bq []rg.Range
	loader := CheckRangeWithFallback[string](store, recorder("ch", &ch), recorder("bq", &bq))

	res, err := loader(context.Background(), rg.NewRange(50, 150))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ch", "bq"}, res)
	require.Len(t, ch, 1)
	assert.Equal(t, rg.NewRange(100, 150), ch[0])
	require.Len(t, bq, 1)
	assert.Equal(t, rg.NewRange(50, 99), bq[0])
}

func TestCheckRangeWithFallback_FullyBelow(t *testing.T) {
	store := fakeRangeStore{r: rg.NewRange(100, 200)}
	var ch, bq []rg.Range
	loader := CheckRangeWithFallback[string](store, recorder("ch", &ch), recorder("bq", &bq))

	res, err := loader(context.Background(), rg.NewRange(50, 80))
	require.NoError(t, err)
	assert.Equal(t, []string{"bq"}, res)
	assert.Empty(t, ch)
	require.Len(t, bq, 1)
	assert.Equal(t, rg.NewRange(50, 80), bq[0])
}

func TestCheckRangeWithFallback_NilFallbackDegradesToCheckRange(t *testing.T) {
	store := fakeRangeStore{r: rg.NewRange(100, 200)}
	var ch []rg.Range
	loader := CheckRangeWithFallback[string](store, recorder("ch", &ch), nil)

	// Within range: served by primary.
	res, err := loader(context.Background(), rg.NewRange(120, 180))
	require.NoError(t, err)
	assert.Equal(t, []string{"ch"}, res)

	// Below range: CheckRange errors (no fallback to extend coverage downward).
	_, err = loader(context.Background(), rg.NewRange(50, 80))
	require.Error(t, err)
}
