# pipe
A multi pool pipeline for heavy multiprocessing in golang

## Introduction

Library `pipe` implements a multithreaded pipeline which runs in several stacked pools of goroutine, allowing developers to fine tune the number of goroutine used in each part of it workload.

## Features

* Rely on [panjf2000/ants](https://github.com/panjf2000/ants) for goroutine pools, inheriting of its properties
* Provide severals helper function to build a pipeline dealing with various types.

## How does it works

`pipe` works with simple processes of three kinds:

* `func(T) T` which is a basic pipelines over a specific type
* `func(T) <-chan K` which allows to split a pipeline into several sub pipeline
* `func(chan<- K) T` which allows to merge several sub pipelines into the parent one.

Each "sub pipeline" is handle by another goroutine pools, to avoid deadlock.

For instance:

- N Job is processed by several processor, each Job are concurrently processed in Pool 0
- Then each Job splits in M SubJob (total SubJob: N*M), each SubJob are concurrently processed in Pool 1 (Pool 0 routines are still occupied by Job)
- Then each SubJob splits in R Groups (total Groups: N*M*R), each Groups are concurrently processed in Pool 2 (Pool 1 routines are still occupied by SubJob)
- Then Groups merge in their parent SubJob. Pool 2 routines are available to new groups whenever available. Pool 1 routines may continue processing SubJob with merged results
- Then SubJob merge in their parent Job. Pool 1 routines are available to new SubJob whenever available. Pool 0 routines may continue processing Job with merged results
- Then Jobs goes out of scope.

## Why is it useful

This design allows to monitor memory through estimation of how much an average Job, SubJob or any sub routine may cost in terms of memory.
Since those objects exists only when they are processed (or when some of them children are processed), and lives in their own routine scoped in a Pool, we can size the
goroutine Pool accordingly.

Pool sizing can be tuned to be able to mitigate between latency for processing each objects and their memory imprints. If an object requires low CPU but wait a lot (API call),
having a large pool may be a good idea... Respectfully the memory imprint of thus objects. If an object requires high CPU and has no wait, size the pool to cpu count may be a good idea.

However, these are general consideration. As for any performance tuning, you should try and tune.

## Installation

```sh
go get -u github.com/ezian/pipe
```

## How to use

### Engine builder

See [examples/example_test.go](/examples/example_test.go) to see how use the pipe building blocks to create an customizable pipeline engine.



## License

The source code in `pipe` is available under the [MIT License](/LICENSE).

## Dependencies

* [panjf2000/ants](https://github.com/panjf2000/ants) a really simple and efficient goroutine pool implementation
* [samber/lo](https://github.com/samber/lo) nice generics implementation with really useful methods



