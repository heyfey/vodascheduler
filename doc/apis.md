---
tags: voda-scheduler
---

# API Endpoints

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

## Training Service

Service name: `voda-scheduler-svc`
Default port: `55587`

| Method | Endpoint | Body | Description | Example |
|--------|----------|------|-------------|---------|
| DELETE | /training | `string`: A job name to delete | Delete a training job | curl -X DELETE   '10.100.86.93:55587/training' --data '"tensorflow2-keras-mnist-elastic-20221019-045648"' |
| GET | /metrics | | Prometheus metrics | curl -X GET '10.100.86.93:55587/metrics'|

<!--- 
| POST | /training | `string`: Path to YAML specification of a training job | Create a training job | curl -X POST  '10.100.86.93:55587/training' --data '"examples/yaml/tensorflow2/tensorflow2-keras-mnist-elastic.yaml"' | 
-->

## Scheduler

Service name: `scheduler-<GPU_TYPE>-svc`
Default port: `55588`

| Method | Endpoint | Body | Description | Example |
|-----|------|---------|-------------|--|
| GET | /training |  | Get all jobs' status in the scheduler | curl -X GET '10.111.11.10:55588/training'
| PUT | /algorithm | `string`: An algorithm name | Set the scheduling algorithm of the scheduler | curl -X PUT   '10.111.11.10:55588/algorithm' --data '"ElasticFIFO"'
| PUT | /ratelimit | `int`: re-scheduling rate limit in seconds | Set the re-scheduling rate limit of the scheduler | curl -X PUT   '10.111.11.10:55588/ratelimit' --data 30
| GET | /metrics | | Prometheus metrics | curl -X GET '10.111.11.10:55588/metrics'

### Get Job Statuses

`curl -X GET '10.111.11.10:55588/training'`

Outputs:
```
NAME                                                         STATUS     WORKERS    SCHEDULER            WAITING    RUNNING    TOTAL     
tensorflow2-keras-mnist-elastic-20221019-045648              Running    1          nvidia-gtx-1080ti    0s         28s        28s  
```

## Resource Allocator

Service name: `resource-allocator-svc`
Default port: `55589`

| Method | Endpoint | Body | Description | Example |
|-----|------|---------|-------------|--|
| GET | /metrics | | Prometheus metrics | curl -X GET '10.100.86.94:55589/metrics'