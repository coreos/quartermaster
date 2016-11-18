# Introduction
QuaterMaster enables the deployment, installation, and management of a storage
under Kubernetes.  QuaterMaster uses _Operator_ technology to interact with
Kubernetes to manage the orchestration of the storage system to a desired
state.

# Architecture

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
