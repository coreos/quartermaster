# Mock Driver

**Status: Not for production, experimental only**

This driver provides an example deployment of a fake storage server onto Kuberntes.
The image it actually deploys is nginx only for simplicity.

## Requirements

There are no requirements to run a mock deployment.

## Deployment

The file [`examples/mock/cluster.yml`](https://github.com/coreos/quartermaster/blob/master/examples/mock/cluster.yaml)
contains an example deployment.  The file has been setup to show a few examples
of how nodeName or nodeSelector can be used, and how container image can be defined.

```
$ kubectl create -f examples/mock/cluster.yaml
```

No dynamic provisioning or persistent volumes will be created since this is a
fake storage system deployment. You can check the status of the storage cluster,
storage nodes, and pvs by running the following commands:

```
$ kubectl get storagecluster -o yaml
$ kubectl get storagenodes -o yaml
$ kubectl get pods
```

It is also recommended to look at the QM logs.
