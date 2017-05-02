# Swift Driver

**Status: Not for production, experimental only**

This driver provides an example of a Swift cluster deployment.
The cluster is configured with one replica rings and can be used to test the Swift API.

## Deployment

The file [`examples/swift/cluster.yml`](https://github.com/coreos/quartermaster/blob/master/examples/swift/cluster.yaml)
contains an example deployment. For quick testing you can use minikube for a
single node deployment or try [kubernetes-swift](https://github.com/thiagodasilva/kubernetes-swift)
for a 3 node deployment.

```
$ kubectl create -f examples/swift/cluster.yaml
```

No dynamic provisioning or persistent volumes will be created since this is a
fake storage system deployment. All data is being stored inside container and
will be deleted once the containers are deleted.

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

Checkout this asciicast for a demo of deploying quartermaster and swift cluster:

[![asciicast](https://asciinema.org/a/118177.png)](https://asciinema.org/a/118177?speed=2&autoplay=1)