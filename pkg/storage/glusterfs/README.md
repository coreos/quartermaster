# GlusterFS Driver

**Status: Not for production, experimental only**

This driver provides an example deployment of GlusterFS onto Kuberntes.  It
is based on [Heketi](https://github.com/heketi/heketi) provides Kubernetes
with dynamic provisioning of GlusterFS volumes from multiple storage clusters.

## Requirements

To you create at least a three node cluster, you can use
[Matchbox/Bootkube](https://github.com/coreos/matchbox/blob/master/Documentation/bootkube.md)
or
[Kubernetes-CentOS](https://github.com/lpabon/kubernetes-centos).

* At least 2Gi RAM
    * Matchbox/Bootkube: Edit `scripts/libvirt` in Matchbox and change the value
      of `--memory=1024` to `--memory=2048`.
* At least one raw extra disk for use by GlusterFS
    * Matchbox/Bootkube: Edit `scripts/libvirt` in Matchbox and add the switch
      `--disk size=100` after `--disk pool=default,size=6`.
* Although it is currently being investigated how to do this automatically,
ports on the nodes must be opened according to [Infrastructure Requirements](https://github.com/gluster/gluster-kubernetes/blob/master/docs/setup-guide.md#infrastructure-requirements).
It is necessary to open these ports because the containers require (today) a network
using host-net, not cluster net.
    * Not necessary if using Matchbox/Bootkube or Kubernetes-CentOS.
* If the Kubernetes system is running some time of authorization mechanism,
like [RBAC](https://kubernetes.io/docs/admin/authorization/), you may need to setup
a service account called `heketi-service-account` which provides the appropriate
rules for Heketi to run container exec commands successfully. See
[`examples/glusterfs/auth/rbac`](https://github.com/coreos/quartermaster/tree/master/examples/glusterfs/auth/rbac).
    * Needed for Matchbox/Bootkube, but not for Kubernetes-CentOS since the latter has no authorization.
* Nodes need to have the kernel module `dm_thin_pool` loaded
    * Needed for Matchbox/Bootkube, but already installed in Kubernetes-CentOS.

## Deployment

The file [`examples/glusterfs/cluster.yml`](https://github.com/lpabon/quartermaster/blob/master/examples/glusterfs/cluster.yaml)
contains an example GlusterFS deployment using three nodes.  The file has been setup to
work with the [demo Kuberntes deployment](https://github.com/lpabon/kubernetes-centos),
but you can change the number of servers and node names according to your Kuberntes
cluster. There is also an [`examples/glusterfs/aws.yml`](https://github.com/lpabon/quartermaster/blob/master/examples/glusterfs/aws.yaml)
provided as an example. To deploy the GlusterFS storage, type:

```
$ kubectl create -f examples/glusterfs/cluster.yaml
```

You can check the status of the storage cluster, storage nodes, and Heketi by
running the following:

```
$ kubectl get storagecluster -o yaml
$ kubectl get storagenodes -o yaml
$ kubectl get pods
```

Deployment of GlusterFS cluster may take some time.  You may look at the status
of your cluster by looking to see if is ready:

```
$ kubectl get storagecluster gluster -o yaml
```

Once the cluster is ready, a [StorageClass](https://kubernetes.io/docs/user-guide/persistent-volumes/#storageclasses)
will be created automatically to enable dynamic provisioning of volumes from
the GlusterFS cluster.

```
$ kubectl get storageclass
```

### Troubleshooting

To easiest way to check for errors is to check the logs of Heketi and QM.

## Example Application

The file [`examples/glusterfs/nginx-demo.yaml`](https://github.com/coreos/quartermaster/blob/master/examples/glusterfs/apps/nginx-demo.yaml)
contains an example application set which will utilize dynamic provisioning to
create a physical volume. Simply run the following command to start the demo:

```
$ kubectl create -f examples/glusterfs/nginx-demo.yaml
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
