# NFS Driver

**Status: Not for production, experimental only**

This driver provides an example deployment of an NFS server onto Kuberntes.  It
is based on NFS-Ganesha and exports over NFS the `/exports` directory inside the
container.

## Requirements

The container used to deploy NFS-Ganesha starts up rpc.bind and therefore cannot
run multiple versions of this NFS implementation on a single host. Keep in mind
that this is an experimental implementation.

## Deployment

The file [`examples/nfs/cluster.yml`](https://github.com/lpabon/quartermaster/blob/master/examples/nfs/cluster.yaml)
contains an example deployment of two nfs servers.  The file has been setup to
work with the [demo Kuberntes deployment](https://github.com/lpabon/kubernetes-centos),
but you can change the number of servers and node names according to your Kuberntes
cluster.

```
$ kubectl create -f examples/nfs/cluster.yaml
```

Since NFS does not support dynamic provisioning, the deployment will create a
persistent volume (PV) for every entry storage node. You can check the status
of the storage cluster, storage nodes, and pvs by running the following commands:

```
$ kubectl get storagecluster -o yaml
$ kubectl get storagenodes -o yaml
$ kubectl get pods
$ kubectl get pv
```

## Example Application

The file `examples/nfs/nginx-demo.yaml` contains an example application set
which will utilize one of the PVs created.  To run the application type:

```
$ kubectl create -f examples/nfs/nginx-demo.yaml
```

This application runs a set of pods which write a `index.html` file with the
date and the host name. To see the output of the application, determine the NodePort
of the nginx service. Here is an example application output:

```
$ kubectl get svc my-nginx-svc -o yaml | grep nodePort
  - nodePort: 31823
$ curl http://<ip of a Kubernetes node>:31823
Mon Mar 20 19:37:30 UTC 2017
nfs-busybox-2t80g
```
