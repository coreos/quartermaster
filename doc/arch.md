# Introduction
QuarterMaster enables the deployment, installation, and management of a specific
storage system onto a Kubernetes cluster.  QuarterMaster uses _Operator_
technology to interact with Kubernetes to manage the orchestration of the
storage system.  The operator will abstract the underlying management storage
system implementation complexity and present the administrator with a simple
storage management system which is fully integrated with Kubernetes.

By abstracting the complexity in the deployment of storage systems,
QuarterMaster will make it possible to easily, and reliably, deploy and
manage a desired storage system in a Kubernetes cluster.

# Architecture
An administrator enables QuarterMaster through a Kubernetes Deployment.
The deployment will start the _QuarterMaster Operator_ Pod which will
determine the desired state of the requested storage system.  If the state
is not satisfied, then QuarterMaster will determine a path to move the
storage system to the desired state.

QuarterMaster will also register two _Third Party Resources_ if they do
not exist: StorageNode, StorageStatus.  _StorageNode_ is a third party
resource which describes a specific storage node.  Administrators will
register one StorageNode object for each of the storage nodes to be managed
by QuarterMaster.  Administrators will then be able to get status of their
storage system by requesting a _StorageStatus_ object from Kubernetes.  This
object will have been updated by QuarterMaster with information describing
the current state of the storage system being managed.

QuarterMaster Pod architecture is divided into two layers.  One layer
is the generic abstraction of storage management which communicates
with Kubernetes.  This layer will be responsible for providing all
the necessary routines required by all storage systems. The second
layer is a storage system specific layer which will be responsible
for the management, control, and communication with the desired
storage system interfaces.

## Namespace Recommendation
A single QuarterMaster operator will be responsible for a single storage
cluster.  It is recommended to have single namespace per quatermaster
deployment. QuarterMaster has no technical restrictions on supporting
multiple storage systems on the same namespace, but it will be simpler
for the administrator to manage their systems through namespace segragation.

## Installation and Usage
The administrator first installs QuarterMaster by submitting the appropriate
deployment for their desired storage system.  For example to deploy Torus,
the administrator would type:

```
$ kubectl create -f quatermaster-torus.yml
```

On an initial setup, QuarterMaster will notice that the storage cannot
be created until storage nodes are defined by the administrator.  At this
point, QuarterMaster will wait for an event from Kubernetes notifying it when
a storage node has been registered with the system.  The status of the storage
system will be set to 'OFFLINE'.  When a new storage node is present,
QuarterMaster storage system specific layer will use this information to
deploy and setup the storage on the specified node.  QuarterMaster will
then update the storage status with this new node information once
the storage system specific layer has finished adding the new node to
the storage cluster.

Administrator can get status of the storage system by inspecting a
_StorageStatus_ third party resource from Kubernetes.  First they would
need to determine the object name, then output its content.

```
$ kubectl get storagestatus
$ kubectl get storagestatus/<storage status object> -o yaml
```

When enough nodes and devices are available, QuarterMaster will change
the status of the storage system to 'ONLINE', according to the requirements
from the storage system specific layer.  An 'OFFLINE' to 'ONLINE' status
transition will enable QuarterMaster to submit a _StorageClass_ object,
which will enable Kubernetes Dynamic Provisioning using the storage
cluster just created.  Dynamic provisioniong will enable users to
have their storage requests automatically satisfied by Kubernetes.


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
