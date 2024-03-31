package pipe_test

import (
	"testing"

	"github.com/fogfactory/pipe"
	"github.com/maxatome/go-testdeep/td"
	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
)

func InitPool(t testing.TB, poolSizes ...int) *pipe.Pools {
	return InitPoolWithOptions(t, poolSizes)
}

func InitPoolWithOptions(t testing.TB, poolSizes []int, opts ...ants.Option) *pipe.Pools {
	pools, err := pipe.NewPoolsWithOptions(poolSizes, opts...)
	td.Require(t).CmpNoError(err)
	t.Cleanup(pools.Release)
	return pools
}

func TestPool(t *testing.T) {
	inc := func(i, _ /* for compatibility with lo */ int) int {
		i++
		return i
	}

	t.Run("pipe_nil_pool", func(t *testing.T) {
		// Arrange
		input := lo.Range(10)
		in := lo.SliceToChannel(0, input)
		do := func(p *pipe.Pools, i int) int {
			td.CmpNil(t, p, "Pool should be nil")
			return inc(i, 0)
		}

		// Act
		out := pipe.Pipe(nil, in, do) // Do will be done sequentially in a gouroutine

		// Assert
		results := lo.ChannelToSlice(out)
		td.Cmp(t, results, lo.Map(input, inc))
	})

	t.Run("pipe_empty_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t) // Empty pool
		input := lo.Range(10)
		in := lo.SliceToChannel(0, input)
		do := func(p *pipe.Pools, i int) int {
			td.CmpLen(t, p.Pools(), 0, "Shouldn't have underlying pool")
			return inc(i, 0)
		}

		// Act
		out := pipe.Pipe(pool, in, do) // Do will be done sequentially in a gouroutine, since the "pools" doesn't contains any pool

		// Assert
		results := lo.ChannelToSlice(out)
		td.Cmp(t, results, lo.Map(input, inc))
	})

	t.Run("pipe_pool_size_1", func(t *testing.T) {
		// Arrange
		pool := InitPool(t, 1)
		input := lo.Range(10)
		in := lo.SliceToChannel(0, input)
		do := func(p *pipe.Pools, i int) int {
			td.CmpLen(t, p.Pools(), 0, "Shouldn't have underlying pool")
			return inc(i, 0)
		}

		// Act
		out := pipe.Pipe(pool, in, do) // Do will be done sequentially in a sub-goroutine, since the "pools" contains only one pool with One goroutine

		// Assert
		results := lo.ChannelToSlice(out)
		td.Cmp(t, results, lo.Map(input, inc))
	})

	t.Run("pipe_pool_size_10", func(t *testing.T) {
		// Arrange
		pool := InitPool(t, 10)
		input := lo.Range(10)
		in := lo.SliceToChannel(0, input)
		do := func(_ *pipe.Pools, i int) int {
			return inc(i, 0)
		}

		// Act
		out := pipe.Pipe(pool, in, do) // Results can come in disorder

		// Assert
		results := lo.ChannelToSlice(out)
		// We use a bag since values can be in disorder, since the pool contains several worker
		td.CmpBag(t, results, lo.Map(input, func(i, _ int) any { return do(nil, i) }))
	})

	t.Run("pipe_pool_with_sub_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t, 1, 1)
		input := lo.Range(10)
		in := lo.SliceToChannel(0, input)
		do := func(p *pipe.Pools, i int) int {
			td.Require(t).NotNil(p, "Pool shouldn't be nil")
			td.CmpLen(t, p.Pools(), 1, "Should have an underlying pool")
			return inc(i, 0)
		}

		// Act
		out := pipe.Pipe(pool, in, do)

		// Assert
		results := lo.ChannelToSlice(out)
		td.Cmp(t, results, lo.Map(input, inc))
	})
}
