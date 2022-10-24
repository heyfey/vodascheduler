---
tags: voda-scheduler
---

# Prometheus Metrics Exposed

Get in-cluster IP of the services:
```
kubectl get service --namespace voda-scheduler
```
Outputs:
```
NAME                              TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                        AGE
mongodb-svc                       ClusterIP   10.104.54.239    <none>        27017/TCP                      6m58s
rabbitmq                          ClusterIP   10.102.102.177   <none>        5672/TCP,15672/TCP,15692/TCP   6m58s
rabbitmq-nodes                    ClusterIP   None             <none>        4369/TCP,25672/TCP             6m58s
resource-allocator-svc            ClusterIP   10.100.86.94     <none>        55589/TCP                      6m58s
scheduler-nvidia-gtx-1080ti-svc   ClusterIP   10.111.11.10     <none>        55588/TCP                      6m58s
scheduler-nvidia-tesla-v100-svc   ClusterIP   10.98.8.220      <none>        55588/TCP                      6m58s
voda-scheduler-svc                ClusterIP   10.100.86.93     <none>        55587/TCP                      6m58s
```

## Scheduler

`curl 10.111.11.10:55588/metrics` for `scheduler-nvidia-gtx-1080ti-svc`

`curl 10.98.8.220:55588/metrics` for `scheduler-nvidia-tesla-v100-svc`

or

`bash scripts/sched_get.sh /metrics`

| Metric name | Metric type | Description |
| -------- | -------- | -------- |
| voda_scheduler_<GPU_TYPE>_scheduler_info | Gauge | Information about the scheduler. |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_created_total     | Counter     | Counts number of training jobs created.     |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_deleted_total     | Counter     | Counts number of training jobs deleted.     |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_completed_total     | Counter     | Counts number of training jobs completed.     |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_failed_total     | Counter     | Counts number of training jobs failed.     |
| voda_scheduler_<GPU_TYPE>_scheduler_resched_total     | Counter     | Counts number of rescheduling.     |
| voda_scheduler_<GPU_TYPE>_scheduler_resched_duration_seconds     | Summary     | A summary of the duration of rescheduling.     |
| voda_scheduler_<GPU_TYPE>_scheduler_resched_allocator_duration_seconds     | Summary     | A summary of the duration of getting scheduling result from resource allocator.     |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_ready     | Gauge     | Number of ready jobs.     |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_waiting     | Gauge     | Number of waiting jobs.     |
| voda_scheduler_<GPU_TYPE>_scheduler_jobs_running     | Gauge     | Number of running jobs.     |
| voda_scheduler_<GPU_TYPE>_scheduler_gpus     | Gauge     | Number of schedulable GPUs.     |
| voda_scheduler_<GPU_TYPE>_scheduler_gpus_inuse     | Gauge     | Number of GPUs in use     |
| voda_scheduler_<GPU_TYPE>_scheduler_placement_algorithm_duration_seconds | Summary | A summary of the duration of placement algorithm. |
| voda_scheduler_<GPU_TYPE>_scheduler_placement_workers_migrated | Gauge | Number of deleted worker pods for migration in last rescheduling. |
| voda_scheduler_<GPU_TYPE>_scheduler_placement_launchers_deleted | Gauge | Number of deleted launcher pods in last rescheduling. |
| voda_scheduler_<GPU_TYPE>_scheduler_placement_jobs_cross_node | Gauge | Number of job that need cross-node communication. |

## Resource Allocator

`curl 10.100.86.94:55589/metrics`

or 

`bash scripts/allocator_get.sh /metrics`

| Metric name | Metric type | Description | Labels |
| -------- | -------- | -------- | --- |
| voda_scheduler_resource_allocator_info | Gauge | Information about the resource allocator. | `namespace`=`voda-scheduler` <br> `version`=<version> |
| voda_scheduler_resource_allocator_database_duration_seconds | Summary | A summary of the duration of fetching information from database. | |
| voda_scheduler_resource_allocator_num_ready_jobs | Summary | A summary of the number of ready jobs of the request. | |
| voda_scheduler_resource_allocator_num_gpus | Summary | A summary of the number of GPUs of the request. | |
| voda_scheduler_resource_allocator_scheduling_algorithm_duration_seconds | Summary | A summary of the duration of scheduling algorithm. | |
| voda_scheduler_resource_allocator_labeled_num_ready_jobs | Summary | A summary of the number of ready jobs of the request. | `algorithm`=<algorithm-name>  |
| voda_scheduler_resource_allocator_labeled_num_gpus | Summary | A summary of the number of GPUs of the request. | `algorithm`=<algorithm-name>  |
| voda_scheduler_resource_allocator_labeled_scheduling_algorithm_duration_seconds | Summary | A summary of the duration of scheduling algorithm. | `algorithm`=<algorithm-name> |
