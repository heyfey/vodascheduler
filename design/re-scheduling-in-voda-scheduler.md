# Re-scheduling in Voda Scheduler

Voda scheduler uses event-driven scheduling.  At every re-scheduling event, the scheduler comes out with a scheduling plan of `<job_name: # of GPUs>` and adjusts from the old plan to the new plan.  Following are steps of a re-scheduling: 

1. An re-scheduling event occurs.  Such as new job arrives or job completed.
2. The scheduler consults the resource allocator for new scheduling plan with all existing jobs.
3. The scheduler also gathers run-time metrics of the jobs for the resource allocator.
4. The resource allocator (where [scheduling algorithms](https://github.com/heyfey/vodascheduler/tree/main/pkg/algorithm) are implemented) comes out with new scheduling plan (i.e. # of GPUs to each job) by putting all existing jobs and their run-time metrics into consideration.
5. The scheduler adjusts from the old scheduling plan to the new plan by resumes(starts)/scales/preempts existing jobs.

![](https://i.imgur.com/lHsbTFJ.png)


## Snippet

The following snippet shows the main logic of the scheduler:

```go
func (s *Scheduler) Run() {
    for {
        select {
        case _ = <-s.ReschedCh:
            s.resched()
        case _ = <-s.StopSchedulerCh:
            // stop the scheduler
            return
        }
    }
}
```

The scheduler listens to the re-scheduling events, and simply send an object to the channel (`ReschedCh`) when they happens.  The re-scheduling events includes: 
1. job arrived
2. job completed
3. (running) job deleted/unrecoverably failed
4. job's priority changed (for scheduling algorithms with priorities defined, e.g. [Tiresias](https://github.com/heyfey/vodascheduler/blob/main/pkg/algorithm/tiresias.go) & [Elastic-Tiresias](https://github.com/heyfey/vodascheduler/blob/main/pkg/algorithm/elastic_tiresias.go))

## Beware of racing

Each kind of event is monitored/triggered by an independent goroutine, which means many goroutines may try to access global information of the scheduler and existing jobs at the same time. 
Make sure to use lock (`Lock()`/`Unlock()` or `RLock()`/`RUnlock()`) properly in these cases to prevent race condition.