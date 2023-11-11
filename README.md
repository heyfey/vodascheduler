---
tags: voda-scheduler
---

# Voda Scheduler

> Note that everything is experimental and may change significantly at any time.

Voda scheduler is a GPU scheduler for elastic deep learning workloads based on [Kubernetes](https://github.com/kubernetes/kubernetes), [Kubeflow Training Operator](https://github.com/kubeflow/training-operator) and [Horovod](https://github.com/horovod/horovod).


Voda Scheduler is designed to be easily deployed in any Kubernetes cluster. For more architectural details, see [design](https://github.com/heyfey/vodascheduler/blob/main/doc/design/voda-scheduler-design.md).

---

Contents
- [Why Elastic Training?](#Why-Elastic-Training)
- [Why Voda Scheduler?](#Why-Voda-Scheduler)
- [Demo](#Demo)
- [Get Started](#Get-Started)
- [Scheduling Algorithms](#Scheduling-Algorithms)
- [Docker Images](#Docker-Images)
- [Prometheus Metrics Exposed](#Prometheus-Metrics-Exposed)
- [Related Projects](#Related-Projects)
- [Reference](#Reference)

## Why Elastic Training?

Elastic training enables the distributed training jobs to be scaled up and down dynamically at runtime, without interrupting the training process.

With elastic training, the scheduler can make training jobs utilize idle resources if there are any and make the most efficient resource allocations if the cluster is heavily-loaded, thus increasing cluster throughput and reducing overall training time.

For more information about elastic training, see [Elastic Horovod](https://horovod.readthedocs.io/en/stable/elastic_include.html), [Torch Distributed Elastic](https://pytorch.org/docs/stable/distributed.elastic.html) or [Elastic Training](https://github.com/skai-x/elastic-training).

## Why Voda Scheduler?

Voda Scheduler provides several critical features for elastic deep learning workloads as follows:

- Rich [Scheduling Algorithms](#Scheduling-Algorithms) (with resource elasticity) to choose from
- [Topology-Aware Scheduling & Worker Migration](https://github.com/heyfey/vodascheduler/blob/main/doc/design/placement-management.md)
    -  Actively consolidate resources to maximize cluster throughput
    -  Particularly important for elastic training since resource allocations can be dynamically adjusted
- Node Addition/Deletion Awareness
    - Co-works with existing autoscaler
    - Makes the best use of spot instances that may come and go with little warning
    - Tolerates failing nodes
- Fault-Tolerance

## Demo

Checkout the [demo](https://youtu.be/M1sUd_-0LnQ) to see how resource allocations are dynamically adjusted (and how worker pods are migrated) to maximize cluster throughput

## Prerequisite

A Kubernetes cluster, on-cloud or on-premise, that can [schedule GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/). Voda Scheduler is tested with `v1.20`

## [Get Started](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md)

1. [Config Scheduler](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md#Config-Scheduler)
2. [Deploy Scheduler](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md#Deploy-Scheduler)
3. [Submit Training Job to Scheduler](https://github.com/heyfey/vodascheduler/blob/main/doc/get-started.md#Submit-Training-Job-to-Scheduler)
4. [API Endpoints](https://github.com/heyfey/vodascheduler/blob/main/doc/apis.md)


## Scheduling Algorithms


| Algorithm | Elastic | Reference |
| -------- | -------- | -------- |
| FIFO   |     |      |
| Elastic-FIFO (default)     | :heavy_check_mark:    |      |
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
- [kubeflow/mpi-operator](https://github.com/kubeflow/mpi-operator): Kubernetes Operator for MPI-based applications (distributed training, HPC, etc.)
- [horovod/horovod](https://github.com/horovod/horovod): Distributed training framework for TensorFlow, Keras, PyTorch, and Apache MXNet.
- [heyfey/munkres](https://github.com/heyfey/munkres): Hungarian algorithm used in the placement algorithm.
- [heyfey/nvidia_smi_exporter](https://github.com/heyfey/nvidia_smi_exporter): nvidia-smi exporter for Prometheus. For monitoring GPUs in the cluster.

## Reference

T. -T. Hsieh and C. -R. Lee, "Voda: A GPU Scheduling Platform for Elastic Deep Learning in Kubernetes Clusters," 2023 IEEE International Conference on Cloud Engineering (IC2E), Boston, MA, USA, 2023, pp. 131-140, doi: 10.1109/IC2E59103.2023.00023. [https://ieeexplore.ieee.org/document/10305838](https://ieeexplore.ieee.org/document/10305838)

```
@INPROCEEDINGS{10305838,
  author={Hsieh, Tsung-Tso and Lee, Che-Rung},
  booktitle={2023 IEEE International Conference on Cloud Engineering (IC2E)}, 
  title={Voda: A GPU Scheduling Platform for Elastic Deep Learning in Kubernetes Clusters}, 
  year={2023},
  volume={},
  number={},
  pages={131-140},
  doi={10.1109/IC2E59103.2023.00023}}
```
