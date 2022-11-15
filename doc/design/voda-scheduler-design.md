---
tags: voda-scheduler
---

# Voda Scheduler Design

Voda scheduler offers a system for scheduling elastic deep learning workloads. It implements several scheduling algorithms that are specifically designed for elastic deep learning. The scheduling algorithm can also be customized at ease.

## Concepts

**Schedule MPIJob.** [Horovod](https://github.com/horovod/horovod) is a popular distributed/elastic training framework for TensorFlow, Keras, PyTorch, and Apache MXNet, which can be easily deployed in kubernetes cluster as MPIJob. Currently, Voda scheduler supports scheduling MPIJob. We plan to support PyTorchJob in the future to support [torch.distributed.elastic](https://pytorch.org/docs/stable/distributed.elastic.html). Both MPIJob and PyTorchJob are controlled by [Kubeflow Training Operator](https://github.com/kubeflow/training-operator).

**Event-Driven Re-scheduling.** On every re-scheduling event, the scheduler comes out with a scheduling plan of `<job_name: # of GPUs>` and adjusts from the old plan to the new plan. In other word, at every re-scheduling, a training job is possible to be started, scaled up, scaled down, or preempted.

The re-scheduling event includes:
1. job arrived
2. job completed
3. (running) job deleted/unrecoverably failed
4. job's priority changed (for scheduling algorithms with priorities defined, e.g. Tiresias & Elastic-Tiresias)
5. computing node added/deleted

The following diagram shows the flow of re-scheduling:
![](https://camo.githubusercontent.com/1e6cf54ced8fcd2a8c3f864d11cc3af48819de24a2f2a393d8cf040ee4c76304/68747470733a2f2f692e696d6775722e636f6d2f6c48736254464a2e706e67)

**Execution Time Prediction** is used by many scheduling algorithms to enhance performance. Voda scheduler collects runtime metrics of running jobs, estimates their remaining execution time, and makes scheduling decisions accordingly. 

**Scheduling in Heterogeneous GPU clusters.** Voda scheduler deploys separated sub-scheduler for each kind of GPU. Users are required to specify what kind of GPU should be used for the training job.

## Architecture Overview

Voda scheduler adopts microservice architecture, consisting of several loosely coupled components:

![](https://i.imgur.com/IWkHRbl.png)

Voda scheduler builds on Kubernetes as a system for deploying, scaling, and managing complex systems. It relies on Kubeflow Training Operator as MPIJob controller.

In above diagram, three sub-schedulers are deployed to schedule jobs using either CPU, Nvidia GTX 1080Ti, or Nvidia Tesla V100 for training. 

