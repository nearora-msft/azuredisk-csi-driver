# Shared disk

## Feature Status: GA

[Azure shared disks](https://docs.microsoft.com/en-us/azure/virtual-machines/windows/disks-shared) is a new feature for Azure managed disks that enables attaching an Azure managed disk to multiple virtual machines (VMs) simultaneously. Attaching a managed disk to multiple VMs allows you to either deploy new or migrate existing clustered applications to Azure.

### Note

Only raw block device(`volumeMode: Block`) is supported on shared disk feature, Kubernetes application should manage coordination and control of writes, reads, locks, caches, mounts, fencing on the shared disk which is exposed as raw block device. **Multi-node read write is not supported by common file systems (e.g. ext4, xfs), it's only supported by cluster file systems.**


###  Example
1. Create Storage Class and PVC

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: managed-csi
provisioner: disk.csi.azure.com
parameters:
  skuName: Premium_LRS
  maxShares: "2"
  cachingMode: None  # ReadOnly cache is not available for premium SSD with maxShares>1
reclaimPolicy: Delete
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-azuredisk
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 256Gi  # minimum size of shared disk is 256GB (P15)
  volumeMode: Block
  storageClassName: managed-csi
```

2. Create a deployment with 2 replicas and specify `volumeDevices`, `devicePath` in Spec

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: deployment-azuredisk
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      name: deployment-azuredisk
    spec:
      containers:
        - name: deployment-azuredisk
          image: mcr.microsoft.com/oss/nginx/nginx:1.17.3-alpine
          volumeDevices:
            - name: azuredisk
              devicePath: /dev/sdx
      volumes:
        - name: azuredisk
          persistentVolumeClaim:
            claimName: pvc-azuredisk
```

3. Check block device in pod

```console
# kubectl exec -it deployment-sharedisk-7454978bc6-xh7jp bash
root@deployment-sharedisk-7454978bc6-xh7jp:/# dd if=/dev/zero of=/dev/sdx bs=1024k count=100
100+0 records in
100+0 records out
104857600 bytes (105 MB, 100 MiB) copied, 0.0502999 s, 2.1 GB/s
```
