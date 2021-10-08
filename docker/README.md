# Voda Scheduler Docker Images

## Repositories

Separate images are provided for different components of Voda scheduler, and are published
to separate repos in DockerHub.

* `heyfey/vodascheduler` The scheduler.
* `heyfey/voda-metrics-collector` The metrics collector.
* `heyfey/mpi-operator` mpi-operator.
* `heyfey/horovod:sha-6916985-tf2.3.0-torch1.6.0-py3.7-cuda10.1-blacklist-recovery` Custom build of Horovod that recover blacklisted hosts after timeout period of 60 seconds (by default).

## Building Custom Images

Building the Docker images should be run from the root vodascheduler directory. For example:

```
docker build -f docker/vodascheduler/Dockerfile .
```

```
docker build -f docker/metrics-collector/Dockerfile .
```
