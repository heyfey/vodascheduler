# NFS PersistentVolume 

ref: 
https://github.com/polyaxon/polyaxon-examples/tree/master/azure
https://ithelp.ithome.com.tw/articles/10196428
https://www.hwchiu.com/kubernetes-storage-ii.html
https://stackoverflow.com/questions/63461731/unable-to-mount-nfs-on-kubernetes-pod

`cat /etc/hosts` to check NAS ip (NFS server url), mine is `192.168.20.100`

`df -hT` ro see if NFS is mounted

`showmount -e 192.168.20.100`
shows:
```
Export list for nas:
/volume1/homes 192.168.20.5,192.168.20.4,192.168.20.3,192.168.20.2,192.168.20.1
```

## Pod

```
apiVersion: v1
kind: Pod
metadata:
  name: apiserver
spec:
  containers:
  - name: apiserver
    image: zxcvbnius/docker-demo
    ports:
      - name: api-port
        containerPort: 3000
    volumeMounts:
      - name: nfs-volumes
        mountPath: /tmp
  volumes:
    - name: nfs-volume
      nfs:
        server: 192.168.20.100
        path: /volume1/homes/heyfey
```

### PV/PVC/Pod

### PV
```
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nfs
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteMany
  nfs:
    server: 192.168.20.100
    path: "/volume1/homes/heyfey"
```

### PVC
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  resources:
    requests:
      storage: 1Gi
```

### Pod
```
apiVersion: v1
kind: Pod
metadata:
  name: apiserver
spec:
  containers:
  - name: apiserver
    image: zxcvbnius/docker-demo
    ports:
      - name: api-port
        containerPort: 3000
    volumeMounts:
      - name: nfs-volumes
        mountPath: /tmp
  volumes:
  - name: nfs-volumes
    persistentVolumeClaim:
     claimName: nfs
```
