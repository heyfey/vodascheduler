---
tags: voda-scheduler
---

# Get Started

First of all,
```
git clone https://github.com/heyfey/vodascheduler.git
cd vodascheduler
```
Then,
1. [Config Scheduler](#Config-Scheduler)
2. [Deploy Scheduler](#Deploy-Scheduler)
3. [Submit Training Job to Scheduler](#Submit-Training-Job-to-Scheduler)


## Config Scheduler

### Label the nodes

Label all schedulable nodes with `vodascheduler/accelerator=<GPU_TYPE_OF_THE_NODE>`

```
kubectl label nodes <NODE_NAME> vodascheduler/accelerator=<GPU_TYPE_OF_THE_NODE>
```

For example, suppose we have two nodes: `gpu5` and `gpu1`. `gpu5` has Nvidia GTX 1080Ti and `gpu1` has Nvidia Tesla V100:
```
kubectl label nodes gpu5 vodascheduler/accelerator=nvidia-gtx-1080ti
kubectl label nodes gpu1 vodascheduler/accelerator=nvidia-tesla-v100
```

Voda scheduler uses the label to identify schedulable nodes and their GPUs
- To add/remove a node to/from Voda Scheduler, simply label/un-label the node.
- Voda scheduler automatically detects how many GPUs are available on labeled nodes.

> Limitation: 
> Currently voda scheduler assumes there is only one kind of GPU in each node. Having multiple kinds of GPU in one node is not supported.

### Taint the nodes

Taint all schedulable nodes with `vodascheduler/hostname=<NODE_NAME>:NoExecute`

```
kubectl taint node <NODE_NAME> vodascheduler/hostname=<NODE_NAME>:NoExecute
```

For example:
```
kubectl taint node gpu5 vodascheduler/hostname=gpu5:NoExecute
kubectl taint node gpu1 vodascheduler/hostname=gpu1:NoExecute
```

This is for topology-aware scheduling and worker migration, see [Taints and Tolerations for Placement Manager](https://github.com/heyfey/vodascheduler/tree/main/deploy/patch-file-tolerations).


## Deploy Scheduler 

### Deploy RabbitMQ Cluster Operator

Voda scheduler uses RabbitMQ as one of its components. To deploy RabbitMQ, first, we need to deploy RabbitMQ cluster operator:
```
kubectl apply -f "https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml"
```
Ref: [Installing RabbitMQ Cluster Operator in a Kubernetes Cluster](https://www.rabbitmq.com/kubernetes/operator/install-operator.html)

### Create `StorageClass` Resource for RabbitMQ

There are several choices for storage in kubernetes. We take NFS as example here.

```bash
helm repo add nfs-subdir-external-provisioner https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/

helm install nfs-subdir-external-provisioner nfs-subdir-external-provisioner/nfs-subdir-external-provisioner \
    --set nfs.server=x.x.x.x \
    --set nfs.path=/exported/path
```

See [Storage Classes#NFS](https://kubernetes.io/docs/concepts/storage/storage-classes/#nfs) and [Kubernetes NFS Subdir External Provisioner](https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner) for more details.

To create `StorageClass` using other than NFS, see [Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/).

### Generate Scheduler Manifests

In `Makefile`, set the GPU types in your cluster to schedule. GPU types should match the node labels you applied before:
```
# Voda scheduler deploys GPU scheduler for each specified GPU types
# GPU type should match node label: vodascheduler/accelerator=<GPU_TYPE_OF_THE_NODE>
## TODO: have the following set
GPU_TYPES = nvidia-gtx-1080ti nvidia-tesla-v100
```

Then,
```
make gen-scheduler
```

This will generate scheduler manifests under `helm/voda-scheduler/templates/scheduler/`

### Deploy Voda Scheduler

Voda scheduler can be deployed via helm:

```
helm install voda-scheduler ./helm/voda-schdeuler \
    --set mongo.persistentVolume.nfs.server=x.x.x.x \
    --set mongo.persistentVolume.nfs.path=/exported/path/for/mongodb \
    --set metricscollector.persistentVolume.nfs.server=x.x.x.x \
    --set metricscollector.persistentVolume.nfs.path=/exported/path/for/metrics
```

Check if deployed successfully:
```
kubectl get all --namespace voda-scheduler
```

Outputs:
```
NAME                                               READY   STATUS        RESTARTS   AGE
pod/metrics-collector-1665107940-x7l9t             1/1     Completed     0          52s
pod/mongodb                                        1/1     Running       0          4m58s
pod/rabbitmq-server-0                              1/1     Running       0          4m57s
pod/rabbitmq-server-1                              1/1     Running       0          4m57s
pod/rabbitmq-server-2                              1/1     Running       0          4m57s
pod/resource-allocator-59d855687d-b974g            1/1     Running       1          4m57s
pod/resource-allocator-59d855687d-fnszt            1/1     Running       1          4m57s
pod/scheduler-nvidia-gtx-1080ti-6c54f9b778-qk95d   1/1     Running       2          4m57s
pod/scheduler-nvidia-tesla-v100-777fd46449-p2ndh   1/1     Running       2          4m57s
pod/training-service-c58c88c4c-cglc5               1/1     Running       2          4m57s
pod/training-service-c58c88c4c-h5sct               1/1     Running       2          4m57s

NAME                             TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                        AGE
service/mongodb-svc              ClusterIP   10.111.132.30   <none>        27017/TCP                      4m58s
service/rabbitmq                 ClusterIP   10.110.82.119   <none>        15692/TCP,5672/TCP,15672/TCP   4m57s
service/rabbitmq-nodes           ClusterIP   None            <none>        4369/TCP,25672/TCP             4m57s
service/resource-allocator-svc   ClusterIP   10.100.86.94    <none>        55589/TCP                      4m58s
service/voda-scheduler-svc       ClusterIP   10.100.86.93    <none>        55587/TCP                      4m58s

NAME                                          READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/resource-allocator            2/2     2            2           4m57s
deployment.apps/scheduler-nvidia-gtx-1080ti   1/1     1            1           4m58s
deployment.apps/scheduler-nvidia-tesla-v100   1/1     1            1           4m58s
deployment.apps/training-service              2/2     2            2           4m57s

NAME                                                     DESIRED   CURRENT   READY   AGE
replicaset.apps/resource-allocator-59d855687d            2         2         2       4m57s
replicaset.apps/scheduler-nvidia-gtx-1080ti-6c54f9b778   1         1         1       4m57s
replicaset.apps/scheduler-nvidia-tesla-v100-777fd46449   1         1         1       4m57s
replicaset.apps/training-service-c58c88c4c               2         2         2       4m57s

NAME                               READY   AGE
statefulset.apps/rabbitmq-server   3/3     4m57s

NAME                                     COMPLETIONS   DURATION   AGE
job.batch/metrics-collector-1665107940   1/1           52s        52s

NAME                              SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
cronjob.batch/metrics-collector   */1 * * * *   False     1        57s             4m57s

NAME                                    ALLREPLICASREADY   RECONCILESUCCESS   AGE
rabbitmqcluster.rabbitmq.com/rabbitmq   True               True               4m57s
```


#### Clean up

```
helm delete voda-scheduler
```

## Submit Training Job to Scheduler

We use `vodascheduler/examples/` for elastic training job examples (including [training scripts](https://github.com/heyfey/vodascheduler/tree/main/examples/py) and [YAMLs](https://github.com/heyfey/vodascheduler/tree/main/examples/yaml)). 

### Config training job

TBD

### Submit training job
```
./bin/cmd create -f ./examples/yaml/tensorflow2/tensorflow2-keras-mnist-elastic.yaml
```

outputs:

```
Training job created: tensorflow2-keras-mnist-elastic-20211015-035048
View your logs by:
    kubectl logs tensorflow2-keras-mnist-elastic-20211015-035048-launcher
```

### Get job statuses

```
./bin/cmd get jobs 
```

outputs:

```
NAME                                                         STATUS     WORKERS    SCHEDULER  WAITING    RUNNING    TOTAL     
tensorflow2-keras-mnist-elastic-20211015-035048              Running    1          default    0s         28s        28s  
```

### Monitor training job

Since the training job is deployed as MPIJob, you can monitor the job the same way as monitor regular MPIJob.

### Delete training job

```
./bin/cmd delete tensorflow2-keras-mnist-elastic-20211015-035048
```

outputs:

```
Training job deleted: tensorflow2-keras-mnist-elastic-20211015-035048
```


