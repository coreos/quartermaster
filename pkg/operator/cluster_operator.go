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
	qmclient "github.com/coreos-inc/quartermaster/pkg/client"
	"github.com/coreos-inc/quartermaster/pkg/spec"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
)

type StorageOperator interface {
	Setup(stopc <-chan struct{}) error
	HasSynced() bool
}

type StorageClusterOperator struct {
	op  *Operator
	inf cache.SharedIndexInformer
}

func NewStorageClusterOperator(op *Operator) StorageOperator {
	return &StorageClusterOperator{
		op: op, // this is a bad idea, but ok for PoC
		inf: cache.NewSharedIndexInformer(
			NewStorageClusterListWatch(op.rclient),
			&spec.StorageCluster{}, resyncPeriod, cache.Indexers{}),
	}
}

func (s *StorageClusterOperator) Setup(stopc <-chan struct{}) error {
	s.inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(p interface{}) {
			s.op.logger.Log("msg", "enqueueStorageCluster", "trigger", "storagecluster add")
			s.add(p.(*spec.StorageCluster))
		},
		DeleteFunc: func(p interface{}) {
			s.op.logger.Log("msg", "enqueueStorageCluster", "trigger", "storagecluster del")
			s.delete(p.(*spec.StorageCluster))
		},

		// This needs to change to get old and new
		UpdateFunc: func(old, new interface{}) {
			s.op.logger.Log("msg", "enqueueStorageCluster", "trigger", "storagecluster update")
			s.update(old.(*spec.StorageCluster), new.(*spec.StorageCluster))
		},
	})

	go s.inf.Run(stopc)

	return nil
}

func (s *StorageClusterOperator) HasSynced() bool {
	return s.inf.HasSynced()
}

func (s *StorageClusterOperator) add(cs *spec.StorageCluster) error {
	s.op.logger.Log("msg", "adding")

	/*
		// Defult stuff
		condition := spec.StorageClusterCondition{}
		condition.Message = "My message 0"
		condition.Reason = "Zero"
		condition.Type = spec.ClusterConditionOffline

		sp.Status.Ready = false
		sp.Status.Message = "This is a message"
		sp.Status.Reason = "I have my reasons"
		sp.Status.Conditions = []spec.StorageClusterCondition{
			condition,
		}

	*/

	// Set default values
	cs.Status.Ready = false

	// Call plugin
	_, err := s.op.storage.AddCluster(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	// Update cluster object
	sclient := qmclient.NewStorageClusters(s.op.rclient, cs.Namespace)
	cs, err = sclient.Update(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	// Submit each node
	s.submitNodesFor(cs)

	return nil

}

func (s *StorageClusterOperator) update(
	old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	s.op.logger.Log("msg", "update")

	if old.ResourceVersion == new.ResourceVersion {
		s.op.logger.Log("msg", "same version")
		return nil
	}

	s.op.logger.Log("msg", "update found")

	// Call plugin
	err := s.op.storage.UpdateCluster(old, new)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	return nil
}

func (s *StorageClusterOperator) delete(cs *spec.StorageCluster) error {
	s.op.logger.Log("msg", "delete")

	// Create client
	ns_client := qmclient.NewStorageNodes(s.op.rclient, cs.GetNamespace())

	// Get all storagenodes with the same label
	list, err := ns_client.List(api.ListOptions{})
	if err != nil {
		return s.op.logger.Log("err", err)
	}
	s.op.logger.Log("nodes to delete", len(list.Items))

	// Delete all nodes
	for _, node := range list.Items {
		if node.Spec.ClusterRef.Name == cs.GetName() {
			err := ns_client.Delete(node.GetName(), nil)
			if err != nil {
				s.op.logger.Log("msg", "unable to delete", "node", node.GetName(), "err", err)
			} else {
				s.op.logger.Log("msg", "deleted", "node", node.GetName())
			}
		}
	}

	// Call plugin
	err = s.op.storage.DeleteCluster(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	return nil
}

func (s *StorageClusterOperator) submitNodesFor(cs *spec.StorageCluster) {
	s.op.logger.Log("msg", "submitting nodes")

	// Create client
	ns_client := qmclient.NewStorageNodes(s.op.rclient, cs.Namespace)

	// Create a reference object
	clusterRef, err := api.GetReference(cs)
	if err != nil {
		s.op.logger.Log("err", err)
		return
	}

	// Create nodes
	for _, ns := range cs.Spec.StorageNodes {

		// Setup StorageNode object
		node := &spec.StorageNode{
			TypeMeta: unversioned.TypeMeta{
				Kind:       "StorageNode",
				APIVersion: cs.APIVersion,
			},
			ObjectMeta: api.ObjectMeta{
				GenerateName: cs.Name + "-",
				Namespace:    cs.Namespace,
				Labels:       cs.Labels,
			},
			Spec: ns,
		}
		node.Spec.ClusterRef = clusterRef

		// Submit the node
		result, err := ns_client.Create(node)
		if err != nil {
			s.op.logger.Log("msg", "unable to create a storage node", "err", err)
		} else {
			s.op.logger.Log("msg", "created a storage node", "name", result.GetName())
		}
	}
}
