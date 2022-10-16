---
tags: voda-scheduler
---

# Placement Management

- [Motivation](#Motivation)
- [Design](#Design)
- [Algorithm](#Algorithm)

## Motivation

A distributed training job usually has several workers. Because the cost of inter-node communication is much higher than that of intra-node communication, it is typically beneficial that all workers of a training job are placed in as few physical nodes as possible.

Therefore, to enhance cluster throughput and reduce job completion time, effective placement management, including locality-aware scheduling and worker migration, is needed.

## Design

### Preparation

To enable the placement manager to fully control the node bindings of all worker pods, we need to taint all nodes with `vodascheduler/hostname=<node_name>:NoExecute` and let the placement manager manage the tolerations of the worker pods.

Also see [Taints and Tolerations for Placement Manager](https://github.com/heyfey/vodascheduler/blob/main/deploy/taints-and-tolerations-for-placement-manager.md)

### Workflow

![](https://i.imgur.com/l3B8x4Z.png)

When a worker pod is created by MPI Operator, it will be without any toleration by default, thus kube-scheduler is unable to bind the pod to any node. As a result, the pod will be in `pending` phase. The placement manager maintains a map of `<pod_name:node_name>` that indicates the node on which each worker pod is scheduled onto. The placement manager uses informer to monitor if there are any pod is created, looks up the pod on the map, and updates the pod with corresponding toleration added using Kubernetes client APIs. Therefore, the worker pod now can be correctly scheduled by kube-scheduler onto the assigned node.

**Worker migration.** When the placement manager comes out with a new placement plan, it compares the new plan against the old one. If the assigned node of a worker pod is found different in two plans, the worker requires a migration. The placement manager simply deletes these pods. The changes will be detected by the MPI Operator. After that, the MPI Operator will re-create the worker pods to meet the MPIJob’s declaration. The processes of the re-created pods are the same as above.

Noted that we use the term migration is actually a simplification. Not like the migration of VM, Voda scheduler simply kills the running pods and replaces them with new ones. The training job wouldn’t be blocked completely due to fault tolerance in elastic training, and the migrated worker pods re-join training when they are re-created and in `running` phase again.


## Algorithm

See the [thesis](https://ndltd.ncl.edu.tw/cgi-bin/gs32/gsweb.cgi/ccd=tuT7lS/record?r1=1&h1=0) section 3.4.2 for details.
