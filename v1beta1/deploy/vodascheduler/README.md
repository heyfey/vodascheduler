# Voda Scheduler Components

This folder contains essential components of Voda scheduler:

- `namespace.yaml`: namespace
- `mpi-operator`: mpi-operator
- `metrics-pvc.yaml`: PV and PVC for run-time metrics of training jobs
- `metrics-collector.yaml`: the metrics collector
- `mongodb.yaml`: database for job info
- `vodascheduler.yaml`: the scheduler and its RBAC and service configs

## Action needed (Important)

### In `metrics-pvc.yaml`

```
apiVersion: v1
kind: PersistentVolume
spec:
  nfs:
    server: 192.168.20.100
    path: "/volume1/homes/heyfey/voda/metrics"
```

You should replace spec of the persistent volume with your own
***Note that this spec must be the same as in [vodascheduler/deploy/users-pv/metrics-pvc-default.yaml](https://github.com/heyfey/vodascheduler/blob/main/deploy/users-pv/metrics-pvc-default.yaml)***

### In `mongodb.yaml`

```
apiVersion: v1
kind: PersistentVolume
spec:
  nfs:
    server: 192.168.20.100
    path: "/volume1/homes/heyfey/voda/mongodb"
```

You should replace spec of the persistent volume with your own