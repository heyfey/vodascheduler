# Voda Design

Voda scheduler offers a system for scheduling elastic training jobs, in which the scheduling algorithms can be customized at ease.  A number of state-of-the-art scheduling algorithms that designed specifically for elastic training to reduced job completion time and increase cluster efficiency are also provided.

## Voda architecture

Voda scheduler is designed to be cloud-native and uses a microservices architecture. Each Voda component is containerized and deployed as regular pod in Kubernetes. It also leverages existing open-sourced projects at best effort for reliability, flexibility and maintainability.

![](https://i.imgur.com/sqTwD6G.png)


1. A **user** creates an elastic training job that is defined in the *MPIJob* Spec in YAML, and submits it to the training service using REST-level HTTP request.
2. The **training service (job master)** reads the MPIJob Spec, admits the request, and dispatches it to the Voda scheduler. It also assigns a unique job name to each job, which is used to track the job among by all components.
3. The **Voda scheduler** maintains an internal job pool, and performs re-scheduling upon the occurrence of important events, including job arrival, job completion, job deletion, and priority change. Two key components are included in the scheduler:
    - **Resource Allocator**: which decides the allocations of resources, such as the number of GPUs.
    - **Placement Manager**: decides the placement of training jobs. Once the scheduling decisions are made, the Voda scheduler will resume/scale/preempt training jobs by creating/updating/deleting MPIJob objects.
4. [**Kubeflow MPI Operator**](https://github.com/kubeflow/mpi-operator), i.e., the MPIJob controller, provides life cycle management of MPIJob. It carries out the scheduling decisions for Voda by managing launcher pod and worker pods of MPIJobs in the cluster.
5. The Voda scheduler co-exists with the default **kube-scheduler**. Thus it uses taints and tolerations to force kube-scheduler to place computing tasks (worker pods) onto the assigned node, determined by the placement manager.
6. **Storage** is where training dataset, training logs, trained model, and runtime metrics are stored. It could be any shared storage such as NFS, HDFS or cloud storage.
7. The **metrics collector** periodically pulls runtime metrics of ongoing training jobs under current resource allocations and updates Job information. It also tracks training processes and estimates the remaining time of ongoing training jobs base on per-epoch execution time.
8. **Job Info (MongoDB)** is a database that stores the history information of training jobs, such as scaling characteristics, remaining time, etc. These information is used by the Voda scheduler to decide the optimal resource allocations.
9.  **Prometheus** is an optional component to monitor cluster resource utilization and exported metrics from training jobs and the Voda scheduler.




