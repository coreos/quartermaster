# Introduction
The deployment and management of distrubted storage systems is not only
complicated, but also quite different accross different storage systems.
Although some distributed storage systems have their own deployment tools,
they are based on a bare metal or virtual machine model and do not integrate
with Kubernetes.  QuarterMaster enables the deployment, installation, and
management of a specific storage system onto a Kubernetes cluster.
QuarterMaster uses _Operator_ technology to interact with Kubernetes to
manage the orchestration of the storage system.  The operator will abstract
the underlying management storage system implementation complexity and
present the administrator with a simple storage management system which is
fully integrated with Kubernetes.

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
It is recommended to have single namespace per quatermaster
deployment. QuarterMaster has no technical restrictions on supporting
multiple storage systems on the same namespace, but it will be simpler
for the administrator to manage their systems through namespace segragation.

## Usage
The administrator first installs QuarterMaster by submitting the appropriate
deployment for their desired storage system.  For example to deploy Torus,
the administrator would type:

```
$ kubectl create -f quatermaster-torus.yml
```

> NOTE: There may be a need to _name_ the specific deployment which will
> also be used as a label for the system.  This is still TBD.

On an initial setup, QuarterMaster will notice that the storage cannot
be created until storage nodes are defined by the administrator.  At this
point, QuarterMaster will wait for an event from Kubernetes notifying it when
a storage node for this storage type has been registered with the system.
The status of the storage system will be set to 'OFFLINE'.  When a new
storage node is present, QuarterMaster storage system specific layer will
use this information to setup the storage on the specified node.
QuarterMaster will then update the storage status with this new node
information once the storage system specific layer has finished adding
the new node to the storage cluster.

Administrator can get status of the storage system by inspecting a
_StorageStatus_ third party resource from Kubernetes.  First they would
need to determine the object name, then output its content.

```
$ kubectl get storagestatus
$ kubectl get storagestatus/<storage status object> -o yaml
```

> NOTE: It would be great for `kubectl get storagestatus` to correctly
> display the appropriate columns with ONLINE/OFFLINE and any other
> future requests.

When enough nodes and devices are available, QuarterMaster will change
the status of the storage system to 'ONLINE', according to the requirements
from the storage system specific layer.  An 'OFFLINE' to 'ONLINE' status
transition will enable QuarterMaster to submit a _StorageClass_ object,
which will enable Kubernetes Dynamic Provisioning using the storage
cluster just created.  Dynamic provisioniong will enable users to
have their storage requests automatically satisfied by Kubernetes.

## Third Party Resources
QuarterMaster will use third party resources called _StorageNodes_ and
_StorageStatus_ to determine how to manage the storage system.  These
resources will also be used by QuarterMaster to save any metadata needed
to manage the storage systems approprietly.

### StorageNode
A _StorageNode_ third party resource describes a system which QuarterMaster
must manage.  It contains all the necessary information to add the reference
storage node to the storage system. QuarterMaster may also use this object
to save metadata specific to the storage system.

Editing a _StorageNode_ support is left up to the storage system specific
layer to determine if the new change will be accepted.  For example,  users
may add and remove devices from the storage node easily, but may or may not
be able to change the IP address of the storage node.

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
      - <Array of IPs in the storage network>
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
A _StorageStatus_ third party resource describes the status of the storage
system.  Users can use this information to determine the health of their
storage system.

StorageStatus Spec:

```yaml
apiVersion: "coreos.com/v1"
kind: StorageStatus
metadata:
  name: <string>
spec:
  type: <Supported types>
  status: <string>
  storageNodes: <Array of storage nodes managed by this storage deployment>
  glusterfs: <Any GlusterFS specific information>
  torus: <Any Torus specific information>
```

## User Interface
Users and administrators would manage QuarterMaster using `kubectl`.

## API Interface
Services would interface with QuarterMaster using the rest endpoints
created by the third party resources.  These endpoints would enable
the service to manage QuarterMaster through the submission and management
of _StorageNodes_ and _StorageStatus_.

# Related Work
Other technologies used to deploy storage systems, like _ceph-deploy_ and
_gdeploy_, are only setup to install those specific storage systems on
either bare metal systems or virtual machines.  These deployment models
also have no integration with Kubernetes, making them unideal to be used
as deployment platforms.  QuarterMaster may also be compared with OpenStack
Cinder, a block storage provisioner for the open source cloud, or OpenStack
Manila, a shared storage provisioner.  It is more accurate to compare these
OpenStack provisioners with Kubernetes Dynamic Provisioning, since both
statisfy storage requests from users.  As of this writing, there is no
other technology in the industry which satisfies similar goals as
QuarterMaster.
