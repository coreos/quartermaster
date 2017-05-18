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
	"time"

	qmclient "github.com/coreos/quartermaster/pkg/client"
	apierrors "k8s.io/kubernetes/pkg/api/errors"

	"github.com/coreos/quartermaster/pkg/spec"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"

	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

type StorageNodeOperator struct {
	op      *Operator
	events  chan *spec.StorageNode
	nodeInf cache.SharedIndexInformer
	dsetInf cache.SharedIndexInformer
}

func NewStorageNodeOperator(op *Operator) StorageOperator {
	return &StorageNodeOperator{
		op:     op,
		events: make(chan *spec.StorageNode, 200),
		nodeInf: cache.NewSharedIndexInformer(
			NewStorageNodeListWatch(op.rclient),
			&spec.StorageNode{}, resyncPeriod, cache.Indexers{}),
		dsetInf: cache.NewSharedIndexInformer(
			cache.NewListWatchFromClient(op.kclient.Extensions().RESTClient(), "deployments", api.NamespaceAll, nil),
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
	go s.worker()
	go func() {
		<-stopc
		close(s.events)
	}()

	return nil
}

func (s *StorageNodeOperator) enqueueStorageNode(p interface{}) {
	s.op.queue.add(p.(*spec.StorageNode))
}

func (s *StorageNodeOperator) enqueueStorageNodeIf(f func(p *spec.StorageNode) bool) {
	cache.ListAll(s.nodeInf.GetStore(), labels.Everything(), func(o interface{}) {
		if f(o.(*spec.StorageNode)) {
			s.enqueueStorageNode(o.(*spec.StorageNode))
		}
	})
}

func (s *StorageNodeOperator) enqueueAll() {
	cache.ListAll(s.nodeInf.GetStore(), labels.Everything(), func(o interface{}) {
		s.enqueueStorageNode(o.(*spec.StorageNode))
	})
}

func (s *StorageNodeOperator) storageNodeForDeployment(d *extensions.Deployment) *spec.StorageNode {
	key, err := keyFunc(d)
	if err != nil {
		utilruntime.HandleError(logger.LogError("error creating key: %v", err))
		return nil
	}

	// Namespace/Name are one-to-one so the key will find the respective StorageNode resource.
	ss, exists, err := s.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		utilruntime.HandleError(logger.LogError("error getting storage node resource: %v", err))
		return nil
	}
	if !exists {
		return nil
	}
	return ss.(*spec.StorageNode)
}

func (s *StorageNodeOperator) deleteDeployment(o interface{}) {
	d := o.(*extensions.Deployment)
	if sn := s.storageNodeForDeployment(d); s != nil {
		s.enqueueStorageNode(sn)
	}
}

func (s *StorageNodeOperator) addDeployment(o interface{}) {
	d := o.(*extensions.Deployment)
	if sn := s.storageNodeForDeployment(d); s != nil {
		s.enqueueStorageNode(sn)
	}
}

func (s *StorageNodeOperator) updateDeployment(oldo, curo interface{}) {
	old := oldo.(*extensions.Deployment)
	cur := curo.(*extensions.Deployment)

	// Periodic resync may resend the deployment without changes in-between.
	// Also breaks loops created by updating the resource ourselves.
	if old.ResourceVersion == cur.ResourceVersion {
		return
	}

	if sn := s.storageNodeForDeployment(cur); s != nil {
		s.enqueueStorageNode(sn)
	}
}

func (s *StorageNodeOperator) HasSynced() bool {
	return s.nodeInf.HasSynced() && s.dsetInf.HasSynced()
}

func (s *StorageNodeOperator) worker() {
	for event := range s.events {
		if err := s.reconcile(event); err != nil {
			utilruntime.HandleError(logger.LogError("reconciliation failed: %v", err))
		}
	}
}

func (s *StorageNodeOperator) reconcile(sn *spec.StorageNode) error {
	key, err := keyFunc(sn)
	if err != nil {
		return logger.Err(err)
	}

	// Get plugin
	storage, err := s.op.GetStorage(sn.Spec.Type)
	if err != nil {
		return logger.Err(err)
	}

	obj, exists, err := s.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		return logger.Err(err)
	}

	if !exists {
		err := storage.DeleteNode(sn)
		if err != nil {
			return logger.Err(err)
		}

		reaper, err := kubectl.ReaperFor(extensions.Kind("Deployment"), s.op.kclient)
		if err != nil {
			return logger.Err(err)
		}

		err = reaper.Stop(sn.Namespace, sn.Name, time.Minute, api.NewDeleteOptions(0))
		if err != nil {
			return logger.Err(err)
		}

		return nil
	}

	// Use the copy in the cache
	sn = obj.(*spec.StorageNode)

	// DeepCopy CS
	sn, err = storageNodeDeepCopy(sn)
	if err != nil {
		return err
	}

	deployClient := s.op.kclient.Extensions().Deployments(sn.Namespace)
	deployment := &extensions.Deployment{}
	deployment.Namespace = sn.Namespace
	deployment.Name = sn.Name
	obj, exists, err = s.dsetInf.GetStore().Get(deployment)
	if err != nil {
		return logger.Err(err)
	}

	if !exists {
		// Check if the deployment exists

		// Get a deployment from plugin
		deploy, err := storage.MakeDeployment(sn, nil)
		if err != nil {
			return logger.Err(err)
		}

		if _, err := deployClient.Create(deploy); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return logger.LogError("unable to create deployment %v: %v",
					deploy.GetName(), err)
			}
		}

	} else if !sn.Status.Added {
		// Check if the StorageNode has been added

		// Check if the deployment is ready
		deploy := obj.(*extensions.Deployment)
		if deploy.Spec.Replicas != deploy.Status.AvailableReplicas {
			return nil
		}

		// Add node
		updated, err := storage.AddNode(sn)
		if err != nil {
			return logger.Err(err)
		}
		sn.Status.Added = true

		// Update node object
		storagenodes := qmclient.NewStorageNodes(s.op.rclient, sn.GetNamespace())
		if updated != nil {
			updated.Status.Added = true
			_, err = storagenodes.Update(updated)
			if err != nil {
				return logger.Err(err)
			}
		} else {
			_, err = storagenodes.Update(sn)
			if err != nil {
				return logger.Err(err)
			}

		}

	} else {
		// Update deployment and driver

		deploy, err := storage.MakeDeployment(sn, obj.(*extensions.Deployment))
		if err != nil {
			return logger.Err(err)
		}

		// TODO(barakmich): This may be broken for DaemonSets.
		// Will be fixed when DaemonSets do rolling updates.
		if _, err := deployClient.Update(deploy); err != nil {
			return logger.Err(err)
		}

		// Update Node
		_, err = storage.UpdateNode(sn)
		if err != nil {
			return logger.Err(err)
		}
	}

	return nil
}

func storageNodeDeepCopy(ns *spec.StorageNode) (*spec.StorageNode, error) {
	objCopy, err := api.Scheme.DeepCopy(ns)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*spec.StorageNode)
	if !ok {
		return nil, logger.LogError("expected StorageNode, got %#v", objCopy)
	}
	return copied, nil
}
