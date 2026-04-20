package rg

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func newRange(a, b uint64) Range {
	return Range{Start: a, End: &b}
}

func Test_remove(t *testing.T) {
	for si := uint64(0); si <= 10; si++ {
		for ei := si - 1; ei <= 10; ei++ {
			for sj := uint64(0); sj <= 10; sj++ {
				for ej := sj - 1; ej <= 10; ej++ {
					ri := newRange(si, ei)
					rj := newRange(sj, ej)
					rk := ri.Remove(rj)
					for x := uint64(0); x <= 11; x++ {
						ok := si <= x && x <= ei && !(sj <= x && x <= ej)
						//log.Debugf("!!! ri:%s, rj:%s, ri.Remove(rj):%s, x:%d, ok:%v\n", ri, rj, rk, x, ok)
						assert.Equalf(t, ok, rk.Contains(x), "invalid result ri:%s, rj:%s, ri.Remove(rj):%s, x:%d, ok:%v", rj, rj, rk, x, ok)
					}
				}
			}
		}
	}
}

func Test_equal(t *testing.T) {
	assert.Equal(t, true, EmptyRange.Equal(newRange(1, 0)))
	assert.Equal(t, true, newRange(1, 0).Equal(EmptyRange))

	assert.Equal(t, true, Range{Start: 1}.Equal(Range{Start: 1}))
	assert.Equal(t, false, Range{Start: 1}.Equal(Range{Start: 2}))

	for si := uint64(0); si <= 10; si++ {
		for ei := si; ei <= 10; ei++ {
			for sj := uint64(0); sj <= 10; sj++ {
				for ej := sj; ej <= 10; ej++ {
					ri := newRange(si, ei)
					rj := newRange(sj, ej)
					eq := si == sj && ei == ej
					assert.Equalf(t, eq, ri.Equal(rj), "invalid result ri:%s, rj:%s, eq:%v", ri, rj, eq)
				}
			}
		}
	}
}

func Test_include(t *testing.T) {
	assert.Equal(t, true, EmptyRange.Include(newRange(1, 0)))
	assert.Equal(t, true, newRange(1, 0).Include(EmptyRange))

	assert.Equal(t, false, EmptyRange.Include(Range{Start: 0}))
	assert.Equal(t, true, Range{Start: 0}.Include(EmptyRange))

	assert.Equal(t, true, Range{Start: 0}.Include(newRange(0, 0)))
	assert.Equal(t, true, Range{Start: 0}.Include(newRange(0, 1)))
	assert.Equal(t, true, Range{Start: 0}.Include(newRange(1, 1)))
	assert.Equal(t, true, Range{Start: 0}.Include(newRange(1, 2)))
	assert.Equal(t, false, newRange(0, 0).Include(Range{Start: 0}))
	assert.Equal(t, false, newRange(0, 1).Include(Range{Start: 0}))
	assert.Equal(t, false, newRange(1, 1).Include(Range{Start: 0}))
	assert.Equal(t, false, newRange(1, 2).Include(Range{Start: 0}))

	assert.Equal(t, true, newRange(1, 3).Include(newRange(1, 2)))
	assert.Equal(t, true, newRange(1, 3).Include(newRange(1, 3)))
	assert.Equal(t, true, newRange(1, 3).Include(newRange(2, 3)))
	assert.Equal(t, false, newRange(1, 2).Include(newRange(1, 3)))
	assert.Equal(t, true, newRange(1, 3).Include(newRange(1, 3)))
	assert.Equal(t, false, newRange(2, 3).Include(newRange(1, 3)))
}
