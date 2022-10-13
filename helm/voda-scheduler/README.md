---
tags: voda-scheduler
---

# Voda Scheduler Helm Chart

![Version: 0.2.0](https://img.shields.io/badge/Version-0.2.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.2.0](https://img.shields.io/badge/AppVersion-0.2.0-informational?style=flat-square)

Voda Scheduler is a GPU scheduler for elastic/distributed deep learning workloads

## Source Code

* <https://github.com/heyfey/vodascheduler>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| metricscollector.image.pullPolicy | string | `"IfNotPresent"` |  |
| metricscollector.image.repository | string | `"heyfey/voda-metrics-collector"` |  |
| metricscollector.image.tag | string | `"0.2.0"` |  |
| metricscollector.persistentVolume.accessModes[0] | string | `"ReadWriteMany"` |  |
| metricscollector.persistentVolume.nfs.path | string | `"/volume1/homes/heyfey/voda/metrics"` |  |
| metricscollector.persistentVolume.nfs.server | string | `"192.168.20.100"` |  |
| metricscollector.persistentVolume.size | string | `"5Gi"` |  |
| metricscollector.persistentVolume.type | string | `"nfs"` |  |
| metricscollector.resources | object | `{}` |  |
| metricscollector.schedule | string | `"*/1 * * * *"` |  |
| mongo.image.repository | string | `"mongo"` |  |
| mongo.image.tag | string | `"5"` |  |
| mongo.persistentVolume.accessModes[0] | string | `"ReadWriteMany"` |  |
| mongo.persistentVolume.nfs.path | string | `"/volume1/homes/heyfey/voda/mongodb/v1beta1"` |  |
| mongo.persistentVolume.nfs.server | string | `"192.168.20.100"` |  |
| mongo.persistentVolume.size | string | `"5Gi"` |  |
| mongo.persistentVolume.type | string | `"nfs"` |  |
| mongo.resources | object | `{}` |  |
| rabbitmq.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].key | string | `"app.kubernetes.io/name"` |  |
| rabbitmq.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].operator | string | `"In"` |  |
| rabbitmq.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].values[0] | string | `"rabbitmq"` |  |
| rabbitmq.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey | string | `"kubernetes.io/hostname"` |  |
| rabbitmq.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight | int | `90` |  |
| rabbitmq.password | string | `"guest"` |  |
| rabbitmq.replicaCount | int | `3` |  |
| rabbitmq.resources | object | `{}` |  |
| rabbitmq.username | string | `"guest"` |  |
| resourceallocator.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].key | string | `"app.kubernetes.io/name"` |  |
| resourceallocator.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].operator | string | `"In"` |  |
| resourceallocator.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].values[0] | string | `"resource-allocator"` |  |
| resourceallocator.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey | string | `"kubernetes.io/hostname"` |  |
| resourceallocator.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight | int | `90` |  |
| resourceallocator.image.repository | string | `"heyfey/voda-resource-allocator"` |  |
| resourceallocator.image.tag | string | `"0.2.0"` |  |
| resourceallocator.replicaCount | int | `2` |  |
| resourceallocator.resources | object | `{}` |  |
| resourceallocator.service.clusterIP | string | `"10.100.86.94"` |  |
| resourceallocator.service.port | int | `55589` |  |
| resourceallocator.service.type | string | `"ClusterIP"` |  |
| scheduler.image.repository | string | `"heyfey/voda-scheduler"` |  |
| scheduler.image.tag | string | `"0.2.0"` |  |
| scheduler.resources | object | `{}` |  |
| scheduler.resumeEnabled | bool | `true` |  |
| tolerations[0].effect | string | `"NoExecute"` |  |
| tolerations[0].key | string | `"vodascheduler/hostname"` |  |
| tolerations[0].operator | string | `"Exists"` |  |
| trainingservice.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].key | string | `"app.kubernetes.io/name"` |  |
| trainingservice.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].operator | string | `"In"` |  |
| trainingservice.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].values[0] | string | `"training-service"` |  |
| trainingservice.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey | string | `"kubernetes.io/hostname"` |  |
| trainingservice.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight | int | `90` |  |
| trainingservice.image.repository | string | `"heyfey/voda-training-service"` |  |
| trainingservice.image.tag | string | `"0.2.0"` |  |
| trainingservice.replicaCount | int | `2` |  |
| trainingservice.resources | object | `{}` |  |
| trainingservice.service.clusterIP | string | `"10.100.86.93"` |  |
| trainingservice.service.port | int | `55587` |  |
| trainingservice.service.targetPort | int | `55587` |  |
| trainingservice.service.type | string | `"ClusterIP"` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.11.0](https://github.com/norwoodj/helm-docs/releases/v1.11.0)