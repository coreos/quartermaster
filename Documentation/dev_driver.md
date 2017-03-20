# Storage driver Development

To create a driver for Quartermaster, first familiarize yourself with the
[Mock](https://github.com/coreos/quartermaster/blob/master/pkg/storage/mock/README.md) driver.
Deploy it and notice its log messages on the Quartermaster pod.

## Create a new driver

1. To create a new driver, first copy `pkg/storage/mock` directory to a new directory
called `pkg/storage/<new driver>`.
1. Rename `pkg/storage/<new driver>/mock.go` to `pkg/storage/<new driver>/<new driver>.go`
1. Replace all `mock` names in `<new driver>.go`
1. Add a new `StorageTypeIdentifierXXX` in [spec.go](https://github.com/coreos/quartermaster/blob/master/pkg/spec/spec.go) which is the name which identify the driver in `StorageCluster:type`.
1. Add any driver specific metadata to `StorageCluster` and `StorageNode` (see @DRIVER tags in spec.go)
1. Add new driver to `operator.New(...)` in [main.go](https://github.com/coreos/quartermaster/blob/master/cmd/quartermaster/main.go)

Now build and test your driver development.  You will need to create a new
`StorageCluster` yaml file based on [examples/mock/cluster.yaml](https://github.com/coreos/quartermaster/blob/master/examples/mock/cluster.yaml)
and change the `type` to match the new driver `StorageTypeIdentifierXXX` you created.
Deploy and it should still behave as the mock driver.

## Driver Development

The driver interface functions have been documented both in the copied go file
from the mock driver and in [interface.go](https://github.com/coreos/quartermaster/blob/master/pkg/storage/interface.go).
