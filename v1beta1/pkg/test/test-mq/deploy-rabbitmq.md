# Deploy RabbitMQ

Create StorageClass Resource

Take NFS as example
deploy via helm

```
helm repo add nfs-subdir-external-provisioner https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/

helm install nfs-subdir-external-provisioner nfs-subdir-external-provisioner/nfs-subdir-external-provisioner \
    --set nfs.server=x.x.x.x \
    --set nfs.path=/exported/path
```

https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner
https://kubernetes.io/docs/concepts/storage/storage-classes/#nfs

deply rabbitmq
```
kubectl apply deploy/vodascheduler/rabbitmq.yaml
```
`sepc.persistence.storageClassName: <your_storageClassName>`
