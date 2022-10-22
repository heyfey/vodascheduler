---
tags: voda-scheduler
---

# Voda Scheduler

> Note that everything is experimental and may change significantly at any time.

Voda scheduler is a GPU scheduler for elastic deep learning workloads based on [Kubernetes](https://github.com/kubernetes/kubernetes), [Kubeflow Training Operator](https://github.com/kubeflow/training-operator) and [Horovod](https://github.com/horovod/horovod).


Voda Scheduler is designed to be easily deployed in any Kuberneters cluster, for more architectural details, see [design](https://github.com/heyfey/vodascheduler/blob/main/doc/design/voda-scheduler-design.md).

---

Contents
- [Why Elastic Training?](#Why-Elastic-Training?)
- [Why Voda Scheduler?](#Why-Voda-Scheduler?)
- [Get Started](#Get-Started)
- [Scheduling Algorithms](#Scheduling-Algorithms)
- [Docker Images](#Docker-Images)
- [Prometheus Metrics Exposed](#Prometheus-Metrics-Exposed)
- [Related Projects](#Related-Projects)

## Why Elastic Training?

Elastic training enables the distributed training jobs to be scaled up and down dynamically at runtime, without interrupting the training process. With elastic training, the scheduler is able to have training jobs utilize idle resources if there are any, as well as make the most efficient resource allocations if the cluster is heavily-loaded, hence increasing cluster throughput and reducing overall training time.

For more information about elastic training, see [Elastic Horovod](https://horovod.readthedocs.io/en/stable/elastic_include.html) and [Torch Distributed Elastic](https://pytorch.org/docs/stable/distributed.elastic.html).

## Why Voda Scheduler?

Voda Scheduler provides several critical features for elastic deep learning workloads as follows.

### Rich Scheduling Algorithms (with Resource Elasticity)

To fully utilize the strength of resource elasticity, Voda implements the most popular algorithms for scheduling elastic deep learning workloads.

See the [list of algorithms provided](#Scheduling-Algorithms). The system administrators can choose any of them.

You can also implement your own scheduling algorithms. Voda offers functionalities to collect run-time metrics of training jobs that could be useful for scheduling.

### Topology-Aware Scheduling

Job placement is critical to the performance of distributed computing jobs on GPU clusters. Voda scheduler offers **topology-aware scheduling** and **worker migration** to consolidate resources, which minimizes communication overhead and maximizes cluster throughput.

### Node Autoscaling and Fault-Tolerance

Voda scheduler is aware of the addition/removal of computing nodes and makes the best scheduling decision upon it, thus smoothly
- co-works with existing autoscaler.
- makes the best use of spot instances that may come and go with little warning.
- tolerates failing nodes.

### Fault-Tolerance of the Scheduler

- Voda scheduler adopts microservice architecture. 
    - For the training service, there is no single point of failure.
    - For the scheduler, it restarts on failure and restores previous status.
- No training job will be interrupted when any of the Voda scheduler components fails. 

## Prerequisite

A Kubernetes cluster, on-cloud or on-premise, that can [schedule GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/). Voda Scheduler is tested with v1.20.

## [Get Started](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md)

1. [Config Scheduler](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md#Config-Scheduler)
2. [Deploy Scheduler](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md#Deploy-Scheduler)
3. [Submit Training Job to Scheduler](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md#Submit-Training-Job-to-Scheduler)
4. [API Endpoints](https://github.com/heyfey/vodascheduler/blob/main/doc/apis.md)


## Scheduling Algorithms


| Algorithm | Elastic | Reference |
| -------- | -------- | -------- |
| FIFO (default)   |     |      |
| Elastic-FIFO     | :heavy_check_mark:    |      |
| SRJF             |     |      |
| Elastic-SRJF     | :heavy_check_mark:    |      |
| Tiresias         |     | Gu, Juncheng, et al. "Tiresias: A GPU cluster manager for distributed deep learning." 16th USENIX Symposium on Networked Systems Design and Implementation (NSDI 19). 2019. https://www.usenix.org/conference/nsdi19/presentation/gu     |
| Elastic-Tiresias | :heavy_check_mark:    | Wu, Yidi, et al. "Elastic Deep Learning in Multi-Tenant GPU Clusters." IEEE Transactions on Parallel and Distributed Systems (2021). https://ieeexplore.ieee.org/abstract/document/9373916     |
| FfDL Optimizer   | :heavy_check_mark:    | Saxena, Vaibhav, et al. "Effective elastic scaling of deep learning workloads." 2020 28th International Symposium on Modeling, Analysis, and Simulation of Computer and Telecommunication Systems (MASCOTS). IEEE, 2020. https://ieeexplore.ieee.org/abstract/document/9285954     |
| AFS-L            | :heavy_check_mark:    | Shin, Jinwoo, and KyoungSoo Park. "Elastic Resource Sharing for Distributed Deep Learning." (2021) https://www.usenix.org/system/files/nsdi21-hwang.pdf     |


## Docker Images

- [Voda Scheduler Docker Images](https://github.com/heyfey/vodascheduler/tree/main/docker)

## Prometheus Metrics Exposed

- [Prometheus Metrics Exposed](https://github.com/heyfey/vodascheduler/tree/main/doc/prometheus-metrics-exposed.md)

## Related Projects

- [kubeflow/training-operator](https://github.com/kubeflow/training-operator): Training operators on Kubernetes.
- [kubeflow/mpi-operator](https://github.com/kubeflow/mpi-operator): The MPIJob controller.
- [horovod/horovod](https://github.com/horovod/horovod): Distributed training framework for TensorFlow, Keras, PyTorch, and Apache MXNet.
- [heyfey/munkres](https://github.com/heyfey/munkres): Hungarian algorithm used in the placement algorithm.
- [heyfey/nvidia_smi_exporter](https://github.com/heyfey/nvidia_smi_exporter): For monitoring GPUs in the cluster. 
