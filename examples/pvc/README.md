# Persistent Volume for running example jobs

This folder contains pv/pvc for running example elastic/distributed training jobs, see [vodascheduler/examples/yaml](https://github.com/heyfey/vodascheduler/tree/main/examples/yaml) for how them are.

- `templates/repos-pvc.yaml`: for training repositories
- `templates/data-pvc.yaml`: for dataset
- `templates/keras-data-pvc.yaml`: for keras dataset
- `templates/outputs-pvc.yaml`: for model checkpoints and training logs

## Steps

### Modify `values.yaml`

We use NFS by default for the examples. In `values.yaml`:

```yaml
data:
  persistentVolume:
    ## Default use NFS for PV
    ## TODO: Overrides server and path for your own  
    nfs:
      server: 192.168.20.100
      path: "/volume1/homes/heyfey/voda/data"
```

You need to modify `<PV_NAME>.persistentVolume.nfs.server` and `<PV_NAME>.persistentVolume.nfs.path` to your own.
Search for `TODO` in `values.yaml` for required modification.

### helm install

```
helm install voda-examples-pvc examples/pvc
```


### Clean-up

