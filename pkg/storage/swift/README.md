# Swift Driver

**Status: Not for production, experimental only**

This driver provides an example deployment of a Swift All in One cluster.
The SAIO is configured with a one replica ring and can be used to test the Swift API.

## Deployment

The file [`examples/swift/cluster.yml`](https://github.com/coreos/quartermaster/blob/master/examples/mock/cluster.yaml)
contains an example deployment.

```
$ kubectl create -f examples/swift/cluster.yaml
```

No dynamic provisioning or persistent volumes will be created since this is a
fake storage system deployment. All data is being stored inside container and
will be deleted once the container is deleted.

You can check the status of the swift cluster running the following commands:

```
$ kubectl get services
$ curl -i http://<public-node-ip>:<node-port>/healthcheck
```

Or if you have the swift client installed, you can test upload and download

```
$ echo 'hello world' > hw
$ swift -A http://<public-node-ip>:<node-port>/auth/v1.0 -U test:tester -K testing upload c1 hw
$ swift -A http://<public-node-ip>:<node-port>/auth/v1.0 -U test:tester -K list c1
```