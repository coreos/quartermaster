// Copyright 2016 The quartermaster Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"github.com/coreos/quartermaster/pkg/spec"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"

	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

type StorageNodeOperator struct {
	op      *Operator
	queue   *queue
	events  chan *spec.StorageNode
	nodeInf cache.SharedIndexInformer
	dsetInf cache.SharedIndexInformer
}

func NewStorageNodeOperator(op *Operator) StorageOperator {
	return &StorageNodeOperator{
		op:     op,
		events: make(chan *spec.StorageNode, 200),
		nodeInf: cache.NewSharedIndexInformer(
			NewStorageNodeListWatch(op.GetRESTClient()),
			&spec.StorageNode{}, resyncPeriod, cache.Indexers{}),
		dsetInf: cache.NewSharedIndexInformer(
			cache.NewListWatchFromClient(op.GetRESTClient(),
				"deployments", api.NamespaceAll, nil),
			&extensions.Deployment{}, resyncPeriod, cache.Indexers{}),
	}
}

func (s *StorageNodeOperator) Setup(stopc <-chan struct{}) error {
	// Register Handlers
	logger.Debug("register event handlers")
	s.nodeInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(p interface{}) {
			logger.Debug("enqueueStorageNode trigger for storage add")
			s.enqueueStorageNode(p)
		},
		DeleteFunc: func(p interface{}) {
			logger.Debug("enqueueStorageNode trigger for storage del")
			s.enqueueStorageNode(p)
		},

		// This needs to change to get old and new
		UpdateFunc: func(_, p interface{}) {
			logger.Debug("enqueueStorageNode trigger for storage update")
			s.enqueueStorageNode(p)
		},
	})
	s.dsetInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(d interface{}) {
			logger.Debug("addDeployment trigger for deployment add")
			s.addDeployment(d)
		},
		DeleteFunc: func(d interface{}) {
			logger.Debug("addDeployment trigger for deployment delete")
			s.deleteDeployment(d)
		},
		UpdateFunc: func(old, cur interface{}) {
			logger.Debug("addDeployment trigger for deployment update")
			s.updateDeployment(old, cur)
		},
	})

	go s.nodeInf.Run(stopc)
	go s.dsetInf.Run(stopc)
	go func() {
		<-stopc
		close(s.events)
	}()

	return nil
}

func (c *StorageNodeOperator) enqueueStorageNode(p interface{}) {
	c.queue.add(p.(*spec.StorageNode))
}

func (c *StorageNodeOperator) enqueueStorageNodeIf(f func(p *spec.StorageNode) bool) {
	cache.ListAll(c.nodeInf.GetStore(), labels.Everything(), func(o interface{}) {
		if f(o.(*spec.StorageNode)) {
			c.enqueueStorageNode(o.(*spec.StorageNode))
		}
	})
}

func (c *StorageNodeOperator) enqueueAll() {
	cache.ListAll(c.nodeInf.GetStore(), labels.Everything(), func(o interface{}) {
		c.enqueueStorageNode(o.(*spec.StorageNode))
	})
}

func (c *StorageNodeOperator) storageNodeForDeployment(d *extensions.Deployment) *spec.StorageNode {
	key, err := keyFunc(d)
	if err != nil {
		utilruntime.HandleError(logger.LogError("error creating key: %v", err))
		return nil
	}

	// Namespace/Name are one-to-one so the key will find the respective StorageNode resource.
	s, exists, err := c.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		utilruntime.HandleError(logger.LogError("error getting storage node resource: %v", err))
		return nil
	}
	if !exists {
		return nil
	}
	return s.(*spec.StorageNode)
}

func (c *StorageNodeOperator) deleteDeployment(o interface{}) {
	d := o.(*extensions.Deployment)
	if s := c.storageNodeForDeployment(d); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *StorageNodeOperator) addDeployment(o interface{}) {
	d := o.(*extensions.Deployment)
	if s := c.storageNodeForDeployment(d); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *StorageNodeOperator) updateDeployment(oldo, curo interface{}) {
	old := oldo.(*extensions.Deployment)
	cur := curo.(*extensions.Deployment)

	// Periodic resync may resend the deployment without changes in-between.
	// Also breaks loops created by updating the resource ourselves.
	if old.ResourceVersion == cur.ResourceVersion {
		return
	}

	if s := c.storageNodeForDeployment(cur); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (s *StorageNodeOperator) HasSynced() bool {
	return s.nodeInf.HasSynced() && s.dsetInf.HasSynced()
}
