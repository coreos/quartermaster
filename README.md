# Overview
Quartermaster is a framework for managing _containerized storage systems_ like Ceph, GlusterFS, NFS-Ganesha, Rook and others on top of Kubernetes. Quartermaster enables the deployment, installation, and integration of these type of storage systems onto a Kubernetes cluster. Quartermaster abstracts this complexity and presents the client with a simple storage deployment model which is fully integrated with Kubernetes. By simplifying the deployment of storage systems, Quartermaster makes it possible to easily and reliably deploy, upgrade, and get status of a desired storage system in a Kubernetes cluster.  Once deployed, a Quartermaster managed storage system could be used to fulfill persistent volume claim requests.  Quartermaster can also be used to help the setup and testing of PersistentVolumes provided by containerized storage systems deployed in Kubernetes.  Today, Quartermaster supports GlusterFS, but it is designed to easily be extended to support a multitude of storage systems on top of Kubernetes.

# Project status: Alpha
We are looking at maturing the Quartermaster framework as well as adding support for more storage systems

# More information

* [Architecture Document](http://bit.ly/2kikXpF)
* [Demo deployment of GlusterFS cluster](http://bit.ly/2kHUEc7)
* [Project Quartermaster Slides for K8S Storage-SIG](http://bit.ly/2jp5VB9)

# Community

* Mailing list: <TBD Google group>
* IRC: #k8s-quartermaster on Freenode

# Getting Started

## Deploying quartermaster
TBD

## Running quartermaster
TBD

# Developers
Developers please see the following documentation on how to build and participate in the quartermaster project.

* Setting up build environment
* Driver Developer Guide

# Licensing
Quartermaster is licensed under the Apache License, Version 2.0.  See [LICENSE](LICENSE) for the full license text.
