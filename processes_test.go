package pipe_test

import (
	"testing"
	"time"

	"github.com/ezian/pipe"
	"github.com/maxatome/go-testdeep/td"
	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
)

// identity defines a simple helper function, which just return the input and do nothing
func identity[T any](p *pipe.Pools, b T) T {
	return b
}

func TestProcesses(t *testing.T) {

	type Main struct {
		init   int
		result int
	}
	type Branch struct {
		acc int
	}

	t.Run("panic_invalid_dispatcher", func(t *testing.T) {
		// Arrange
		var panicResult any
		pool := InitPoolWithOptions(t, []int{1}, ants.WithPanicHandler(func(i interface{}) { panicResult = i }))
		input := lo.Range(10)
		in := lo.SliceToChannel(0, input)
		mainProc := func(pool *pipe.Pools, i int) int { return i }
		dispatcher, _ := pipe.NewDispatch(func(parent int, in chan<- int) {
			in <- parent // produce two childs
			in <- parent
		}, func(parent int, out <-chan int) int {
			return <-out // consume only one child
		})
		mainProc = pipe.Wrap(mainProc, dispatcher)

		// Act
		pipe.Run(pool, in, mainProc)

		// Assert
		td.CmpContains(t, panicResult, "invalid dispatcher merge int into int, leaked goroutine")
	})

	t.Run("success_linear_process_no_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t)
		in := lo.SliceToChannel(0, []Main{{init: 10}})
		mainProc := pipe.Link(pipe.AsPoolProcesses(
			func(m Main) Main {
				m.init = m.init * 2
				return m
			},
			func(m Main) Main {
				m.result = m.init
				return m
			},
		)...)

		// Act
		out := pipe.Pipe(pool, in, mainProc)

		// Assert
		results := lo.ChannelToSlice(out)
		td.Cmp(t, results, []Main{{init: 20, result: 20}})
	})

	t.Run("success_dispatched_process_no_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t)
		in := lo.SliceToChannel(0, []Main{{init: 10}})
		var result int
		dispatcher, _ := pipe.NewDispatch(func(parent Main, in chan<- Branch) {
			in <- Branch{acc: parent.init} // produce two childs with parent value
			in <- Branch{acc: parent.init}
		}, func(parent Main, out <-chan Branch) Main {
			parent.result = lo.Reduce(lo.ChannelToSlice(out), func(agg int, item Branch, _ int) int {
				agg += item.acc // Sum childs (so result = parent + parent)
				return agg
			}, parent.result)
			return parent
		})
		collectResult := pipe.AsPoolProcess(func(m Main) Main {
			// Just smuggle out result from processes
			result = m.result
			return m
		})
		// Build the process: dispatch 10 in two branches then merge (add 10+10) and finally collect the result)
		process := pipe.Link(
			pipe.Wrap(identity[Branch], dispatcher),
			collectResult)

		// Act
		pipe.Run(pool, in, process)

		// Assert
		td.Cmp(t, result, 20)
	})

	t.Run("success_dispatched_process_with_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t, 1, 2) // 2 goroutine for handling childs
		in := lo.SliceToChannel(0, []Main{{init: 10}})
		var result int
		dispatcher, _ := pipe.NewDispatch(func(parent Main, in chan<- Branch) {
			in <- Branch{acc: parent.init} // produce two childs with parent value
			in <- Branch{acc: parent.init + 1}
		}, func(parent Main, out <-chan Branch) Main {
			parent.result = lo.Reduce(lo.ChannelToSlice(out), func(agg int, item Branch, _ int) int {
				agg += item.acc // Sum childs (so result = parent + parent)
				return agg
			}, parent.result)
			return parent
		})
		collectResult := pipe.AsPoolProcess(func(m Main) Main {
			// Just smuggle out result from processes
			result = m.result
			return m
		})
		topeLa := make(chan bool)
		deadlock := false
		branchProcess := pipe.AsPoolProcess(func(m Branch) Branch {
			// Arbitrary reconciliation to check that both branch are run in separate routine
			select {
			case topeLa <- true:
			case <-topeLa:
			case <-time.After(50 * time.Millisecond):
				deadlock = true
			}
			return m
		})

		// Build the process: dispatch 10 and 11 in two branches then merge (add 10+11) and finally collect the result)
		process := pipe.Link(
			pipe.Wrap(branchProcess, dispatcher),
			collectResult)

		// Act
		pipe.Run(pool, in, process)

		// Assert
		td.Cmp(t, result, 21)
		td.CmpFalse(t, deadlock, "Deadlock detected. Branchs are not runned in several pools")
	})

	t.Run("success_dispatched_process_without_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t, 1, 0) // childs are handled synchroneously with parent
		in := lo.SliceToChannel(0, []Main{{init: 10}})
		dispatcher, _ := pipe.NewDispatch(func(parent Main, in chan<- Branch) {
			in <- Branch{}
			in <- Branch{}
		}, func(parent Main, out <-chan Branch) Main {
			_ = lo.ChannelToSlice(out) // read all slices
			return parent
		})
		topeLa := make(chan bool)
		deadlock := false
		branchProcess := pipe.AsPoolProcess(func(m Branch) Branch {
			// Arbitrary reconciliation to check that both branch are run in separate routine
			// It should dead lock since childs are runned sequentially
			select {
			case topeLa <- true:
			case <-topeLa:
			case <-time.After(10 * time.Millisecond):
				deadlock = true
			}
			return m
		})

		// Build the process
		process := pipe.Wrap(branchProcess, dispatcher)

		// Act
		pipe.Run(pool, in, process)

		// Assert
		td.CmpTrue(t, deadlock, "No deadlock detected. Branchs are runned in several pools.")
	})

	t.Run("success_linear_process_with_pool", func(t *testing.T) {
		// Arrange
		pool := InitPool(t, 2) // 2 goroutine for handling main
		in := lo.SliceToChannel(0, []Main{{}, {}})
		topeLa := make(chan bool)
		deadlock := false
		process := pipe.AsPoolProcess(func(m Main) Main {
			// Arbitrary reconciliation to check that both branch are run in separate routine
			select {
			case topeLa <- true:
			case <-topeLa:
			case <-time.After(50 * time.Millisecond):
				deadlock = true
			}
			// Just smuggle out result from processes
			return m
		})

		// Act
		pipe.Run(pool, in, process)

		// Assert
		td.CmpFalse(t, deadlock, "Deadlock detected. Mains are not runned in several pools")
	})
}

func TestDispatcher(t *testing.T) {

	t.Run("error_invalid_new_dispatcher", func(t *testing.T) {
		// Arrange
		var nilSplit pipe.Split[int, string]
		var nilMerge pipe.Merge[int, string]

		// Act
		dispatcher, err := pipe.NewDispatch(nilSplit, nilMerge)

		// Assert
		td.CmpErrorIs(t, err, pipe.ErrInvalidDispatcher)
		td.CmpErrorIs(t, dispatcher.Validate(), pipe.ErrInvalidDispatcher)
	})

	t.Run("success_new_dispatcher", func(t *testing.T) {
		// Arrange
		split := func(parent string, in chan<- string) {
			// placeholder
		}
		merge := func(parent string, out <-chan string) string {
			return parent // placeholder
		}

		// Act
		dispatcher, err := pipe.NewDispatch(split, merge)

		// Assert
		td.CmpNoError(t, err)
		td.CmpNoError(t, dispatcher.Validate())
	})
}
