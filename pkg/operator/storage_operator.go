package operator

type StorageOperator interface {
	Setup(stopc <-chan struct{}) error
	HasSynced() bool
}
