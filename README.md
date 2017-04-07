# Overview

Quartermaster is a framework for managing _containerized storage systems_ like
Ceph, GlusterFS, NFS-Ganesha, Rook and others on top of Kubernetes. Quartermaster
enables the deployment, installation, and integration of these type of storage
systems onto a Kubernetes cluster. Quartermaster abstracts this complexity and
presents the client with a simple storage deployment model which is fully
integrated with Kubernetes. By simplifying the deployment of storage systems,
Quartermaster makes it possible to easily and reliably deploy, upgrade, and get
status of a desired storage system in a Kubernetes cluster.  Once deployed, a
Quartermaster managed storage system could be used to fulfill persistent volume
claim requests. Quartermaster can also be used to help the setup and testing of
PersistentVolumes provided by containerized storage systems deployed in Kubernetes.
Today, Quartermaster supports GlusterFS, but it is designed to easily be extended
to support a multitude of storage systems on top of Kubernetes.

# Project status: Alpha

Quartermaster is under heavy development. We are looking at maturing the
Quartermaster framework as well as adding support for more storage systems

# More information

* [Architecture Document](http://bit.ly/2kikXpF)
* [Demo deployment of GlusterFS cluster](https://asciinema.org/a/108285)
* [Project Quartermaster Slides for K8S Storage-SIG](http://bit.ly/2jp5VB9)

# Getting Started

## Kubernetes Development Environment Setup

You can use [minikube](https://github.com/kubernetes/minikube) but only for `mock`
or `nfs` storage deployments since minikube is only a single node. To create a
Kubernetes cluster with storage, it is recommended you use
[Matchbox/Bootkube](https://github.com/coreos/matchbox/blob/master/Documentation/bootkube.md)
or [Kubernetes-CentOS](https://github.com/lpabon/kubernetes-centos).  Check driver
documentation for any requirements on these systems.

## Deploying quartermaster

Deploy Quartermaster to the kube-system namespace:

```
kubectl run -n kube-system kube-qm --image=quay.io/lpabon/qm
```

Now that Quartermaster is running, you can deploy one of the following storage
systems onto your Kubernetes cluster.

* [Mock](pkg/storage/mock/README.md): The fake storage system deployment used for developers.
* [NFS](pkg/storage/nfs/README.md): The simplest storage system deployment.
* [GlusterFS](pkg/storage/glusterfs/README.md): A reliable distributed file system.
* [Swift](pkg/storage/swift/README.md): OpenStack Swift object storage

# Developers

Developers please see the following documentation on how to build and participate
in the quartermaster project.

* [Setting up build environment](Documentation/dev_setup.md)
* [Driver Developer Guide](Documentation/dev_driver.md)

# Licensing

Quartermaster is licensed under the Apache License, Version 2.0.  See
[LICENSE](LICENSE) for the full license text.
