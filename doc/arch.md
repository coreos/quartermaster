# Introduction
QuaterMaster enables the deployment, installation, and management of a specific
storage system onto a Kubernetes cluster.  QuaterMaster uses _Operator_
technology to interact with Kubernetes and manage the orchestration of the
storage system.

# Architecture
An administrator deploys QuaterMaster through a Kubernetes Deployment.
The deployment will start the _QuaterMaster Operator_ Pod which will
determine the desired state of the requested storage system.  If the state
is not satisfied, then QuaterMaster will then determine a path to move the
storage system to the desired state.  QuaterMaster will also register
two _Third Party Resources_ if they do not exist: StorageNode, StorageStatus.
_StorageNode_ is a third party resource which describes a storage node,

On an initial setup, QuaterMaster will notice that the storage cannot
be created until storage nodes are defined by the administrator.  At this
point QuaterMaster will wait for an event from Kubernetes showing new
storage nodes have been registered with the system.


## QM Operator

## Third Party Resources

### StorageNode

StorageNode Spec:

```yaml
apiVersion: "coreos.com/v1"
kind: StorageNode
metadata:
  name: <string>
spec:
  type: <Supported types>
  nodeSelector: <map[string]string>
  storageNetwork:
    ips:
      - <Array of IPs>
  devices:
    - <Array of raw devices>
  directories:
    - <Array of directories on node>
  glusterfs: <GlusterFS specific information>
  torus: <Torus specifc information>
```

Example of a StorageNode for GlusterFS:

```yaml
apiVersion: "coreos.com/v1"
kind: StorageNode
metadata:
  name: storage-node1
spec:
  type: glusterfs
  nodeSelector:
    'kubernetes.io/hostname': storage-node1
  storageNetwork:
    ips:
      - 10.10.10.100
  devices:
    - /dev/vdb
    - /dev/vdc
    - /dev/vdd
  glusterfs:
    cluster: cluster1
    zone: 1
```

#### GlusterFS

Description of the GlusterFS subsection of a StorageNode:

```yaml
  glusterfs:
    cluster: <Cluster ID node should belong to>
    zone: <Zone ID, If missing, Zone 1 will be assumed>
```

#### Torus

Description of the Torus subsection of a StorageNode:

```yaml
  torus:
    <TBD>
```

### StorageStatus

StorageStatus Spec:

```yaml
apiVersion: "coreos.com/v1"
kind: StorageStatus
metadata:
  name: <string>
spec:
  type: <Supported types>
  status: <string>
  glusterfs:
  torus:
```

## User Interface

## API Interface

# Design
