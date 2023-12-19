package pipe

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
)

var (
	ErrInvalidDispatcher = errors.New("invalid dispatcher")
)

// Process defines a basic function which update a variable and return the updated variable
type Process[T any] func(T) T

// PoolProcess defines a basic function which update a variable and return the updated variable, but with the capability to parralelize its execution in a Pool
type PoolProcess[T any] func(*Pools, T) T

// Split defines a operation which allow a Parent structure to split into several childs.
type Split[Parent, Child any] func(parent Parent, in chan<- Child)

// Merge defines a operation which allow a Parent to be updated from several childs. The updated parent is returned
type Merge[Parent, Child any] func(parent Parent, out <-chan Child) Parent

// Dispatch combines Split and Merge since there are linked. Basically, in a program flows we will do : parent -(Split)-> [childs...] -(Merge)-> parent
type Dispatch[Parent, Child any] struct {
	split Split[Parent, Child]
	merge Merge[Parent, Child]
}

// NewDispatch creates a Dispatch from Split and Merge functions.
func NewDispatch[Parent, Child any](split Split[Parent, Child], merge Merge[Parent, Child]) (Dispatch[Parent, Child], error) {
	result := Dispatch[Parent, Child]{split, merge}
	return result, result.Validate()
}

// Validate helps to validate Dispatch construct since it should have both fonction initialized
func (d Dispatch[Parent, Child]) Validate() error {
	if d.merge == nil || d.split == nil {
		var p Parent
		var c Child
		return fmt.Errorf("%w from %T to %T (nil fields method: %+v)", ErrInvalidDispatcher, p, c, d)
	}
	return nil
}

// AsPoolProcess decorates a Process, in order to make it seen as a PoolProcess
func AsPoolProcess[T any](proc Process[T]) PoolProcess[T] {
	return func(p *Pools, t T) T { return proc(t) }
}

// AsPoolProcesses is an helper function to call AsPoolProcess on lists
func AsPoolProcesses[T any](procs ...Process[T]) []PoolProcess[T] {
	return lo.Map(procs, func(proc Process[T], _ int) PoolProcess[T] {
		return AsPoolProcess(proc)
	})
}

// LinkP merge several PoolProcess to one
func Link[T any](procs ...PoolProcess[T]) PoolProcess[T] {
	return func(pool *Pools, t T) T {
		return lo.Reduce(procs, func(val T, proc PoolProcess[T], _ int) T { return proc(pool, val) }, t)
	}
}

// Link merge several Process to one
type AnyProcess[T any] interface {
	Process[T] | PoolProcess[T]
}

// Wrap creates a PoolProcess parent from a child pool process and a dispatcher. Child PoolProcess will be called concurrently triggered by dispatcher Split function, then merged into through the dispatcher Merge function
func Wrap[Parent, Child any](procs PoolProcess[Child], dispatch Dispatch[Parent, Child]) PoolProcess[Parent] {
	return func(pool *Pools, p Parent) Parent {

		if err := dispatch.Validate(); err != nil {
			panic(err)
		}

		in := make(chan Child)
		out := Pipe(pool, in, func(pool *Pools, c Child) Child {
			return procs(pool, c)
		})

		go func() {
			defer close(in)
			dispatch.split(p, in)
		}()

		result := dispatch.merge(p, out)

		// confirm that all elements in out channel where consumed
		if val, ok := <-out; ok {
			panic(fmt.Sprintf("invalid dispatcher merge %T into %T, leaked goroutine", val, result))
		}

		return result
	}
}

// Run executes a pool process on a channel and wait until the input channel is closed and the process is terminated
func Run[T any](pool *Pools, in <-chan T, proc PoolProcess[T]) {
	out := Pipe(pool, in, func(pool *Pools, t T) T {
		return proc(pool, t)
	})
	for range out {
		// Nothing to do, we just loop until out is closed
	}
}

// RunAll is a convenient function to run a list of Poolprocess, in order
func RunAll[T any](pool *Pools, in <-chan T, procs []PoolProcess[T]) {
	Run(pool, in, Link(procs...))
}
