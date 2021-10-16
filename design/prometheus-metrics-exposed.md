# Prometheus Metrics Exposed

| Metric name | Metric type | Description |
| -------- | -------- | -------- |
| voda_scheduler_jobs_created_total     | Counter     | Counts number of training jobs created     |
| voda_scheduler_jobs_deleted_total     | Counter     | Counts number of training jobs deleted     |
| voda_scheduler_jobs_completed_total     | Counter     | Counts number of training jobs completed     |
| voda_scheduler_jobs_failed_total     | Counter     | Counts number of training jobs failed     |
| voda_scheduler_resched_total     | Counter     | Counts number of rescheduling     |
| voda_scheduler_resched_duration_seconds     | Summary     | A summary of the duration of rescheduling     |
| voda_scheduler_resched_algorithm_duration_seconds     | Summary     | A summary of the duration of rescheduling algorithm"     |
| voda_scheduler_jobs_waiting     | Gauge     | Number of waiting jobs     |
| voda_scheduler_gpus_inuse     | Gauge     | Number of GPUs in use     |
| voda_scheduler_placement_algorithm_duration_seconds     | Summary     | A summary of the duration of placement algorithm     |

### curl 10.100.86.93:55587/metrics