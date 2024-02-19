package pipe

import (
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
)

// Pools define a slice of in depth pools.
type Pools struct {
	pools []*ants.Pool
}

// Release releases all the pools inside the pools.
func (p *Pools) Release() {
	for _, p := range p.pools {
		if p == nil {
			continue
		}
		p.Release()
	}
}

// NewPoolsWithOptions builds a depth pools with the size in parameters. If there is no size, no pools will be created. Submit will not run in parallel.
//
// Moreover, a size of 0 means that the task pushed at this level will run in their parent routine (or alike).
func NewPoolsWithOptions(poolSizes []int, opts ...ants.Option) (*Pools, error) {
	var err error
	result := &Pools{
		pools: lo.FilterMap(poolSizes, func(size, _ int) (pool *ants.Pool, ok bool) {
			if err != nil {
				return nil, false
			}
			if size != 0 { // if size == 0, it will yield a nil pool, which is OK :  related subprocess will be run in parent process
				pool, err = ants.NewPool(size, opts...)
			}
			return pool, err == nil
		}),
	}
	if err != nil {
		result.Release() // release eventually created pools
		return nil, err
	}
	return result, nil
}

// NewPools builds a depth pools with the size in parameters. If there is no size, no pools will be created. Submit will not run in parallel.
func NewPools(poolSizes ...int) (*Pools, error) {
	return NewPoolsWithOptions(poolSizes)
}

// Pipe allows to Pipe a channel in and out in the depth pool. It will execute the task in the current pool and pass the next level pool to the child task.
func Pipe[IN, OUT any](dp *Pools, in <-chan IN, do func(*Pools, IN) OUT) <-chan OUT {
	out := make(chan OUT)

	go func() {
		var wg sync.WaitGroup
		for dispatch := range in {
			value := dispatch
			wg.Add(1)
			dp.submit(func(dp *Pools) {
				defer wg.Done()
				out <- do(dp, value)
			})
		}
		// Wait for all submitted task were done, to close out channel
		wg.Wait()
		close(out)
	}()

	return out
}

// submit submits a task to the pools. if the remaining pools are empty, it is blocking until the task complete.
func (p *Pools) submit(f func(*Pools)) {
	if p == nil || len(p.pools) == 0 {
		f(p) // If there is no more available pools or no pool at all, just do it in current routine thread
		return
	}
	currentPool := p.pools[0]
	childrenPools := &Pools{pools: p.pools[1:]}
	if currentPool == nil {
		f(childrenPools) // If the current pool is nil, run in the current thread
		return
	}
	err := p.pools[0].Submit(func() { f(childrenPools) })
	if err != nil {
		// FIXME have a proper error handling, even if it shouldn't happens, except for some exotic configuration
		panic(err)
	}
}
