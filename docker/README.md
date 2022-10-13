---
tags: voda-scheduler
---

# Voda Scheduler Docker Images

## Repositories

Separate images are provided for different components of Voda scheduler, and are published
to separate repos in DockerHub.

* `heyfey/voda-training-service` The training service.
* `heyfey/voda-scheduler` The scheduler.
* `heyfey/voda-resource-allocator` The resource allocator.
* `heyfey/voda-metrics-collector` The metrics collector.

## Building Custom Images

Building the Docker images should be run from the root vodascheduler directory. For example:

```
docker build -f docker/training-service/Dockerfile .
```
```
docker build -f docker/scheduler/Dockerfile .
```
```
docker build -f docker/resource-allocator/Dockerfile .
```
```
docker build -f docker/metrics-collector/Dockerfile .
```