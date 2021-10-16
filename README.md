# VodaScheduler

> Note that everything is experimental and may change significantly at any time.

Voda scheduler is a GPU scheduler for elastic deep learning workloads base on [Kubernetes](https://github.com/kubernetes/kubernetes), [Kubeflow MPI Operator](https://github.com/kubeflow/mpi-operator) and [Horovod](https://github.com/horovod/horovod).


Voda Scheduler is developed by NTHU SCOPE Lab, and designed to be easily deployed in any Kuberneters cluster.

For more architectural  details, see [design](https://github.com/heyfey/vodascheduler/blob/main/design/voda-design.md).

Contents
- [Why Elastic Training?](#Why-Elastic-Training?)
- [Why Voda Scheduler?](#Why-Voda-Scheduler?)
- [Quick Start](#Quick-Start)
- [Scheduling Algorithms](#Scheduling-Algorithms)
- [Exposed Metrics](#Exposed-Metrics)
- [Docker Images](#Docker-Images)
- [Related Projects](#Related-Projects)
- [Publications](#Publications)

## Why Elastic Training?

Elastic training enables the distributed training jobs to be scaled up and down dynamically at runtime, without interrupting the training process. With elastic training, the training jobs can utilize idle resources when there is any; and the scheduler can make most efficient resource allocations to the jobs, hence increases cluster throughput and reduces training time.

For more information about elastic training, see [Elastic Horovod](https://horovod.readthedocs.io/en/stable/elastic_include.html).

## Why Voda Scheduler?

Voda Scheduler provides several key features for elastic deep learning workloads as follows.

### State-of-the-art Scheduling Algorithms

To fully utilize the strength resource elastically, Voda implements the most popular algorithms for scheduling elastic deep learning workloads.

See the [list of algorithms provided](#Scheduling-Algorithms). The system administrators can choose any of them.

The scheduling algortihms can also be customized at ease. Voda offers functionalities to collect run-time metrics of training jobs that could be useful for scheduling.

### Topology-Aware Scheduling

Job placement is critical to performance of  distributed training jobs on GPU clusters because of communication cost. Voda scheduler offers **topology-aware scheduling** and **worker migration** for minimizing communication overhead, as well as maximizing cluster throughput.


## Prerequisite

A Kubernetes cluster, on-cloud or on-premise. Voda Scheduler is tested with v1.20.

## Quick Start

```
git clone https://github.com/heyfey/vodascheduler.git
cd vodascheduler
```

### Taints the nodes

Taint all schedulable nodes with `vodascheduler/hostname=<node_name>:NoExecute`

```
kubectl taint node <node_name> vodascheduler/hostname=<node_name>:NoExecute
```

Please see [Taints and Tolerations for Placement Manager](https://github.com/heyfey/vodascheduler/tree/main/deploy/patch-file-tolerations).

### Configure PV/PVC

Follow instrunctions in [Voda Scheduler Components](https://github.com/heyfey/vodascheduler/tree/main/deploy/vodascheduler) and [Persistent Volume for Users](https://github.com/heyfey/vodascheduler/tree/main/deploy/users-pv).

Then, deploy the pv/pvc (for example jobs):
```
kubectl apply -f deploy/users-pv
```


### Run scheduler
```
kubectl apply -f deploy/vodascheduler
```

You can check whether Voda scheduler is deployed via:
```
kubectl get all --namespace voda-scheduler
```

outputs:
```
NAME                                     READY   STATUS      RESTARTS   AGE
pod/metrics-collector-1634360700-sv9k9   0/1     Completed   0          17s
pod/mongodb                              1/1     Running     0          42s
pod/voda-scheduler-55689bbf97-8rp8l      1/1     Running     1          45s

NAME                     TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)     AGE
service/mongodb-svc      ClusterIP   10.99.116.141   <none>        27017/TCP   42s
service/voda-scheduler   ClusterIP   10.100.86.93    <none>        55587/TCP   45s

NAME                             READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/voda-scheduler   1/1     1            1           45s

NAME                                        DESIRED   CURRENT   READY   AGE
replicaset.apps/voda-scheduler-55689bbf97   1         1         1       45s

NAME                                     COMPLETIONS   DURATION   AGE
job.batch/metrics-collector-1634360700   1/1           2s         17s

NAME                              SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
cronjob.batch/metrics-collector   */1 * * * *   False     0        20s             42s
```


### Submitting a training job to scheduler

```
./bin/cmd create -f examples/yaml/tensorflow2/tensorflow2-keras-mnist-elastic.yaml
```

outputs:

```
Training job created: tensorflow2-keras-mnist-elastic-20211015-035048
View your logs by:
    kubectl logs tensorflow2-keras-mnist-elastic-20211015-035048-launcher
```

See `vodascheduler/examples/` for elastic training job examples (including [training scripts](https://github.com/heyfey/vodascheduler/tree/main/examples/py) and [YAMLs](https://github.com/heyfey/vodascheduler/tree/main/examples/yaml)). 

### Getting job statuses

```
./bin/cmd get jobs 
```

outputs:

```
NAME                                                         STATUS     WORKERS    SCHEDULER  WAITING    RUNNING    TOTAL     
tensorflow2-keras-mnist-elastic-20211015-035048              Running    1          default    0s         28s        28s  
```

### Monitoring training jobs

Since the jobs are deployed as MPIJobs, you can monintor the jobs as you monintor regular MPIJobs.

### Deleting a training job

```
./bin/cmd delete tensorflow2-keras-mnist-elastic-20211015-035048
```

outputs:

```
Training job deleted: tensorflow2-keras-mnist-elastic-20211015-035048
```

### Clean up

```
kubectl delete -f deploy/vodascheduler
```

## Scheduling Algorithms

[algorithms](https://github.com/heyfey/vodascheduler/tree/main/pkg/algorithm)

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

## Exposed Metrics

[Prometheus Metrics Exposed](https://github.com/heyfey/vodascheduler/blob/main/design/prometheus-metrics-exposed.md)

## Docker Images

[Voda Scheduler Docker Images](https://github.com/heyfey/vodascheduler/tree/main/docker)

## Related Projects

[kubeflow/mpi-operator](https://github.com/kubeflow/mpi-operator): The MPIJob controller.

[heyfey/horovod](https://github.com/heyfey/horovod/tree/blacklist-recovery-v0.21.3): Distributed training framework (customized version).

[heyfey/munkres](https://github.com/heyfey/munkres): Hungarian algorithm used in the placement algorithm.

[heyfey/nvidia_smi_exporter](https://github.com/heyfey/nvidia_smi_exporter): For monitoring GPUs in the cluster. 

## Publications
