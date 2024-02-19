package benchmark

import (
	"fmt"
	"math"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/ezian/pipe"
	"github.com/samber/lo"
)

// profilepipe generate a profile file. It will be outputted as pipe_{date}_in{childRatio}_{poolsizes}.prof.
//
// - childRatio Number of childs generated at each branching.
// - poolSizes Pool sizes. Its length is also used to define sub branch depth.
//
// use pprof to read the file (go install github.com/google/pprof@latest).
func Profile(childRatio int, poolSizes ...int) {
	// Profile file
	f, err := os.Create(fmt.Sprintf("pipe_%s_in%d_%s.prof",
		strings.ReplaceAll(time.Now().Truncate(time.Second).Format(time.DateTime), " ", "-"),
		childRatio,
		strings.Join(lo.Map(poolSizes, func(item, _ int) string { return fmt.Sprint(item) }), "-")))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Init engine
	pool, _ := pipe.NewPools(poolSizes...)
	in := lo.SliceToChannel(0, []int{1})
	dumbProc := pipe.AsPoolProcess(func(i int) int { time.Sleep(time.Millisecond); return i })
	dumbDispatch, _ := pipe.NewDispatch(func(parent int, in chan<- int) {
		for i := 0; i < childRatio; i++ {
			in <- parent
		}
	}, func(parent int, out <-chan int) int {
		for {
			if _, ok := <-out; !ok { // discard all
				return parent
			}
		}
	})
	process := dumbProc
	for i := 0; i < len(poolSizes); i++ {
		process = pipe.Wrap[int, int](process, dumbDispatch)
	}

	// linear processing equivalent
	// for each
	totalCall := 0
	for i := 0; i < len(poolSizes); i++ {
		totalCall += int(math.Pow(float64(childRatio), float64(i+1)))
	}
	fmt.Println("totalCalls: ", totalCall, ", minimal seq duration:", time.Duration(totalCall)*time.Millisecond)

	// Start profiling
	func() {
		_ = pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

		// Run pipe
		start := time.Now()
		pipe.Run(pool, in, process)
		fmt.Printf("(par: %s)\n", time.Since(start))
	}()

	val := 0

	start := time.Now()
	for i := 0; i < totalCall; i++ {
		val = dumbProc(nil, val)
	}
	fmt.Printf("(seq: %s)\n", time.Since(start))
	fmt.Printf("profile:%s\n", f.Name())

	// Call pprof on a file
	// pprof -http=:8080 $file
	// On all files
	// source <(ls | grep .prof | nl | awk '{print "pprof -http=:"$1 + 8080, $2,$3,"&"}')
}
