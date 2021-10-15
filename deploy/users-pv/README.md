# Persistent Volume for Users

This folder contains example pv/pvc for distributed training with kubernetes, check out [vodascheduler/examples/yaml](https://github.com/heyfey/vodascheduler/tree/main/examples/yaml) for how to use them

***Note that only `metrics-pvc-default.yaml` is a must for Voda Scheduler***

- `metrics-pvc.yaml`: for run-time metrics of training jobs
- `repos-pvc`: for training repositories
- `data-pvc.yaml`: for dataset
- `keras-data-pvc.yaml`: for keras dataset
- `outputs-pvc.yaml`: for model checkpoints and training logs 
- `nfs.yaml`: for all you need if you don't want so many pv/pvc

## Action needed (Important)

Note that all pv/pvc are optional except `metrics-pvc-default.yaml`

### In `metrics-pvc-default.yaml`

```
apiVersion: v1
kind: PersistentVolume
spec:
  nfs:
    server: 192.168.20.100
    path: "/volume1/homes/heyfey/voda/metrics"
```

You should replace spec of the persistent volume with your own
***(This spec must be the same as in [vodascheduler/deploy/vodascheduler/metrics-pvc.yaml]((https://github.com/heyfey/vodascheduler/blob/main/deploy/vodascheduler/metrics-pvc.yaml)))***

### And so on to all other pv/pvc