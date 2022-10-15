---
tags: voda-scheduler
---

# Persistent Volume for running example jobs

This folder contains pv/pvc for running example elastic/distributed training jobs, see [vodascheduler/examples/yaml](https://github.com/heyfey/vodascheduler/tree/main/examples/yaml) for how them are.

- `templates/repos-pvc.yaml`: for training repositories
- `templates/data-pvc.yaml`: for dataset
- `templates/keras-data-pvc.yaml`: for keras dataset
- `templates/outputs-pvc.yaml`: for model checkpoints and training logs

## Steps

### Setup `values.yaml`

We use NFS by default for the examples. To use NFS, we need to setup `values.yaml`, or pass NFS parameters to `helm install` and `helm upgrade`

`values.yaml`:
```yaml
data:
  persistentVolume:
    ## TODO: have the following set     
    nfs:
      server: 192.168.20.100
      path: "/volume1/homes/heyfey/voda/data"
```

You need to have `<PV_NAME>.persistentVolume.nfs.server` and `<PV_NAME>.persistentVolume.nfs.path` set.
Search for `TODO` in `values.yaml` for required settings.

### helm install

From the root vodascheduler directory:

```
helm install voda-examples-pvc ./examples/pvc
```

#### Pass NFS parameters to `helm install` and `helm upgrade`

(Setup an NFS server first. Get its IP as `NFS_IP`)

```
...
--set data.persistence.type=nfs \
--set data.persistence.nfs.server=${NFS_IP} \
--set data.persistence.nfs.path=${PATH_TO_DATASET} \
--set kerasdata.persistence.type=nfs \
--set kerasdata.persistence.nfs.server=${NFS_IP} \
--set kerasdata.persistence.nfs.path=${PATH_TO_KERAS_DATASET} \
--set repo.persistence.type=nfs \
--set repo.persistence.nfs.server=${NFS_IP} \
--set repo.persistence.nfs.path=${PATH_TO_REPO} \
--set outputs.persistence.type=nfs \
--set outputs.persistence.nfs.server=${NFS_IP} \
--set outputs.persistence.nfs.path=${PATH_TO_OUTPUTS} \
...
```

### Clean-up

```
helm delete voda-examples-pvc
```

