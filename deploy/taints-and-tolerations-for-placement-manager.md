# Taints and Tolerations for Placement Manager

The placement manager manages job placements using [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)

## Taint the nodes

Taint all the dedicated nodes with `vodascheduler/hostname={node_name}:NoExecute`

For instances:
```
kubectl taint node node1 vodascheduler/hostname=node1:NoExecute
kubectl taint node node2 vodascheduler/hostname=node2:NoExecute
kubectl taint node node3 vodascheduler/hostname=node3:NoExecute
```

## Add tolerations if needed

Typically we need to add tolerations to `deployment` and `daemonset` if we taint the nodes with `NoExecute`. We provide `patch-file-tolerations.yaml` to simplify the process.

list all deployments:
```
kubectl get deployment -A
```
outputs:
```
NAMESPACE      NAME                                                  READY   UP-TO-DATE   AVAILABLE   AGE
kube-system    coredns                                               2/2     2            2           67d
mpi-operator   mpi-operator                                          1/1     1            1           38h
prometheus     kube-prometheus-stack-1625-operator                   1/1     1            1           6d
prometheus     kube-prometheus-stack-1625125397-grafana              1/1     1            1           6d
prometheus     kube-prometheus-stack-1625125397-kube-state-metrics   1/1     1            1           6d
```
patch the deployments if needed:
```
kubectl patch deployment coredns --patch "$(cat patch-file-tolerations.yaml)" --namespace kube-system
kubectl patch deployment mpi-operator --patch "$(cat patch-file-tolerations.yaml)" --namespace mpi-operator
kubectl patch deployment kube-prometheus-stack-1625125397-grafana --patch "$(cat patch-file-tolerations.yaml)" --namespace prometheus
kubectl patch deployment kube-prometheus-stack-1625125397-kube-state-metrics --patch "$(cat patch-file-tolerations.yaml)" --namespace prometheus
```

list all daemonsets:
```
kubectl get daemonset -A
```
outputs:
```
NAMESPACE     NAME                                                        DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR            AGE
default       dcgm-exporter-1625053931                                    2         2         2       2            2           <none>                   6d20h
kube-system   kube-flannel-ds                                             2         2         2       2            2           <none>                   67d
kube-system   kube-proxy                                                  2         2         2       2            2           kubernetes.io/os=linux   67d
kube-system   nvidia-device-plugin-1619776390                             2         2         2       2            2           <none>                   67d
prometheus    kube-prometheus-stack-1625125397-prometheus-node-exporter   2         2         2       2            2           <none>                   6d
```
patch the daemonsets if needed:
```
kubectl patch daemonset dcgm-exporter-1625053931 --patch "$(cat patch-file-tolerations.yaml)"
kubectl patch daemonset kube-flannel-ds --patch "$(cat patch-file-tolerations.yaml)" --namespace kube-system
kubectl patch daemonset nvidia-device-plugin-1619776390 --patch "$(cat patch-file-tolerations.yaml)" --namespace kube-system
kubectl patch daemonset kube-prometheus-stack-1625125397-prometheus-node-exporter --patch "$(cat patch-file-tolerations.yaml)" --namespace prometheus
```