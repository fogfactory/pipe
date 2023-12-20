package examples

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ezian/pipe"
	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
)

/*
This file contains an example of use pipe to build a Rating Engine.

This rating Engine take "Job" as input and dispatch "SubJob".

It shows how to define builders to make a clean definition of this kind of engine.
*/

func smallType[T any](t T) string {
	return strings.TrimLeft(filepath.Ext(fmt.Sprintf("%T", t)), ".") // remove package name from type
}

// mockproc is a convenient fonction to mock any processing function by doing nothing
func mockproc[T any](t T) T {
	fmt.Printf(">%s>", smallType(t))
	return t
}

// mockSplit is a convenient fonction to mock any split function by doing nothing
func mockSplit[Parent, Child any](childCount int) func(parent Parent, in chan<- Child) {
	return func(parent Parent, in chan<- Child) {
		var child Child
		fmt.Printf("[%s/%s]", smallType(parent), smallType(child))
		for i := 0; i < childCount; i++ {
			in <- child
		}
	}
}

// mockMerge is a convenient fonction to mock any merge function by doing nothing
func mockMerge[Parent, Child any](parent Parent, out <-chan Child) Parent {
	_ = lo.ChannelToSlice(out) // discard any childs
	var child Child
	fmt.Printf(`[%s\%s]`, smallType(child), smallType(parent))

	return parent
}

// Engine defines the engine
type Engine struct {
	pool *pipe.Pools
	proc pipe.PoolProcess[Job]
}

// Job defines the parent job. It will be dispatched in several subjob. It is defined here as an interface for convenience but could be of any type.
type Job struct {
	childCount int
}

// SubJob defines a specific. It is the most atomic item of a rating engine. It is defined here as an interface for convenience but could be of any type.
type SubJob struct{}

// JobBuilder defines the base builder for the whole engine
type JobBuilder struct {
	preproc    []pipe.Process[Job]
	dispatched *pipe.PoolProcess[Job]
	postproc   []pipe.Process[Job]
	poolSizes  []int
	poolOpts   []ants.Option
}

// SubJobBuilder defines a builder which build subpipeline
type SubJobBuilder struct {
	parent *JobBuilder
	split  pipe.Split[Job, SubJob]
	procs  []pipe.Process[SubJob]
}

// PoolSizes defines the poolsize for the underlying pools
func (b *JobBuilder) PoolSizes(sizes ...int) *JobBuilder {
	if len(b.poolSizes) > 0 {
		panic("engine poolsizes defined twice")
	}
	b.poolSizes = sizes
	return b
}

// PoolOpts adds options to all underlying pools
func (b *JobBuilder) PoolOpts(opts ...ants.Option) *JobBuilder {
	b.poolOpts = append(b.poolOpts, opts...)
	return b
}

// Processor add a processor. If a dispatcher has been defined, it will add the processor after the dispatcher
func (b *JobBuilder) Processor(proc pipe.Process[Job]) *JobBuilder {
	if b.dispatched == nil {
		b.preproc = append(b.preproc, proc)
	} else {
		b.postproc = append(b.postproc, proc)
	}
	return b
}

// Split initializes the split function from Job to Subjob
func (b *JobBuilder) Split(split pipe.Split[Job, SubJob]) *SubJobBuilder {
	return &SubJobBuilder{parent: b, split: split}
}

// Processor add a processor to subjob pipeline
func (b *SubJobBuilder) Processor(proc pipe.Process[SubJob]) *SubJobBuilder {
	b.procs = append(b.procs, proc)
	return b
}

// Merge initializes the merge function from Subjob to Job
func (b *SubJobBuilder) Merge(merge pipe.Merge[Job, SubJob]) *JobBuilder {
	dispatcher, err := pipe.NewDispatch(b.split, merge)
	if err != nil {
		panic(err) // Or handle the error by collecting them in parent...
	}
	b.parent.dispatched = lo.ToPtr(pipe.Wrap(pipe.Link(pipe.AsPoolProcesses(b.procs...)...), dispatcher))
	return b.parent
}

// Build returns the engine
func (b *JobBuilder) Build() *Engine {
	if b.dispatched == nil {
		panic("no dispatcher") // Or handle the errors properly ;)
	}
	pool, err := pipe.NewPoolsWithOptions(b.poolSizes, b.poolOpts...)
	if err != nil {
		panic(err) // Or handle the errors properly ;)
	}
	return &Engine{
		pool: pool,
		proc: pipe.Link(pipe.Link(pipe.AsPoolProcesses(b.preproc...)...), *b.dispatched, pipe.Link(pipe.AsPoolProcesses(b.postproc...)...)),
	}
}

// NewEngineBuilder creates an Engine builder
func NewEngineBuilder() *JobBuilder {
	return &JobBuilder{}
}

// Run runs the engine... VROOOOOOOOOOOOOOOOOOOOOOOMMMMMMM !!!
func (e *Engine) Run(in <-chan Job) {
	pipe.Run(e.pool, in, e.proc)
}

func ExampleEngine() {

	// Will generate several childs depending on parent
	split := func(parent Job, in chan<- SubJob) {
		mockSplit[Job, SubJob](parent.childCount)(parent, in)
	}

	engine := NewEngineBuilder().
		PoolSizes(0, 2). // The first pool should be 0 or 1 to keep output lines order. Second pool doesn't have any effect since there is no differencies between subjob output
		Processor(mockproc[Job]).
		Processor(mockproc[Job]).
		Split(split).
		Processor(mockproc[SubJob]).
		Processor(mockproc[SubJob]).
		Merge(mockMerge[Job, SubJob]).
		Processor(mockproc[Job]).
		Processor(func(c Job) Job {
			fmt.Println()
			return c
		}).
		Build()

	jobs := []Job{
		{childCount: 1},
		{childCount: 2},
	}
	engine.Run(lo.SliceToChannel(0, jobs)) // Push 2 job, will output two lines

	// Output:
	// >Job>>Job>[Job/SubJob]>SubJob>>SubJob>[SubJob\Job]>Job>
	// >Job>>Job>[Job/SubJob]>SubJob>>SubJob>>SubJob>>SubJob>[SubJob\Job]>Job>
}
