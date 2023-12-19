/*
pipe allows to run multithreaded pipelines with bounded and stacked pools of goroutine.

It allows to fine tune a pipeline execution by controlling overall memory imprint.

pipe support pipelines splitting into several branches then merging back in the principal branches. Each branch has its own goroutine pool.

For instance:

- N Job is processed by several processor, each Job are concurrently processed in Pool 0
- Then each Job splits in M SubJob (total SubJob: N*M), each SubJob are concurrently processed in Pool 1 (Pool 0 routines are still occuped by Job)
- Then each SubJob splits in R Groups (total Groups: N*M*R), each Groups are concurrently processed in Pool 2 (Pool 1 routines are still occuped by SubJob)
- Then Groups merge in their parent SubJob. Pool 2 routines are available to new groups whenever available. Pool 1 routines may continue processing SubJob with merged results
- Then SubJob merge in their parent Job. Pool 1 routines are available to new SubJob whenever available. Pool 0 routines may continue processing Job with merged results
- Then Jobs goes out of scope.

This design allows to monitor memory through estimation of how much an average Job, SubJob or any sub routine may cost in terms of memory.
Since those objects exists only when they are processed (or when some of them childs are processed), and lives in their own routine scoped in a Pool, we can size the
goroutine Pool accordingly.

Pool sizing can be tuned to be able to mitigate between latency for processing each objects and their memory imprints. If an object requires low CPU but wait a lot (API call),
having a large pool may be a good idea... Respectfully the memory imprint of thus objects. If an object requires high CPU and has no wait, size the pool to cpu count may be a good idea.

However, these are general consideration. As for any performance tuning, you should try and tune.
*/

package pipe
