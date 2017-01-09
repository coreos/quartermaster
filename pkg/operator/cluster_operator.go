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
	"fmt"
	"hash/adler32"

	qmclient "github.com/coreos-inc/quartermaster/pkg/client"
	"github.com/coreos-inc/quartermaster/pkg/spec"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	hashutil "k8s.io/kubernetes/pkg/util/hash"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

type StorageOperator interface {
	Setup(stopc <-chan struct{}) error
	HasSynced() bool
}

type StorageClusterOperator struct {
	op     *Operator
	events chan *spec.StorageCluster
	inf    cache.SharedIndexInformer
}

func NewStorageClusterOperator(op *Operator) StorageOperator {
	return &StorageClusterOperator{
		op:     op, // this is a bad idea, but ok for PoC
		events: make(chan *spec.StorageCluster, 200),
		inf: cache.NewSharedIndexInformer(
			NewStorageClusterListWatch(op.rclient),
			&spec.StorageCluster{}, resyncPeriod, cache.Indexers{}),
	}
}

func (s *StorageClusterOperator) Setup(stopc <-chan struct{}) error {
	s.inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(p interface{}) {
			s.op.logger.Log("msg", "enqueueStorageCluster", "trigger", "storagecluster add")
			s.events <- p.(*spec.StorageCluster)
		},
		DeleteFunc: func(p interface{}) {
			s.op.logger.Log("msg", "enqueueStorageCluster", "trigger", "storagecluster del")
			s.events <- p.(*spec.StorageCluster)
		},

		// This needs to change to get old and new
		UpdateFunc: func(old, new interface{}) {
			s.op.logger.Log("msg", "enqueueStorageCluster", "trigger", "storagecluster update")
			s.update(old.(*spec.StorageCluster), new.(*spec.StorageCluster))
		},
	})

	go s.inf.Run(stopc)
	go s.worker()
	go func() {
		<-stopc
		close(s.events)
	}()

	return nil
}

func (s *StorageClusterOperator) HasSynced() bool {
	return s.inf.HasSynced()
}

func (s *StorageClusterOperator) worker() {
	for event := range s.events {
		if err := s.reconcile(event); err != nil {
			utilruntime.HandleError(fmt.Errorf("reconciliation failed: %s", err))
		}
	}
}

func (s *StorageClusterOperator) reconcile(cs *spec.StorageCluster) error {

	key, err := keyFunc(cs)
	if err != nil {
		return err
	}
	s.op.logger.Log("msg", "reconcile storagenode", "key", key)

	// Determine if it was deleted
	obj, exists, err := s.inf.GetStore().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		return s.delete(cs)
	}

	// Use the copy in the cache
	cs = obj.(*spec.StorageCluster)

	// DeepCopy CS
	out, err := storageClusterDeepCopy(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	// Check if it _will_ be deleted and update the status only

	// Check StorageNodes created for this cluster
	return s.add(out)
}

func storageClusterDeepCopy(cs *spec.StorageCluster) (*spec.StorageCluster, error) {
	objCopy, err := api.Scheme.DeepCopy(cs)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*spec.StorageCluster)
	if !ok {
		return nil, fmt.Errorf("expected StorageCluster, got %#v", objCopy)
	}
	return copied, nil
}

func (s *StorageClusterOperator) add(cs *spec.StorageCluster) error {
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

	// Call plugin
	storage, err := s.op.GetStorage(cs.Spec.Type)
	if err != nil {
		return err
	}

	_, err = storage.AddCluster(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	// Update cluster object
	sclient := qmclient.NewStorageClusters(s.op.rclient, cs.Namespace)
	cs, err = sclient.Update(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	return s.submitNodesFor(cs)
}

func (s *StorageClusterOperator) update(
	old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	s.op.logger.Log("msg", "update")

	if old.ResourceVersion == new.ResourceVersion {
		s.op.logger.Log("msg", "same version")
		return nil
	}

	if old.Spec.Type != new.Spec.Type {
		return s.op.logger.Log("error", "changing storage type is not supported")
	}

	s.op.logger.Log("msg", "update found")

	// TODO(lpabon): need to determine if a storagenode has been removed
	// by comparing old and new
	// s.events <- new

	// Get storage plugin
	storage, err := s.op.GetStorage(old.Spec.Type)
	if err != nil {
		return err
	}

	// Call plugin
	err = storage.UpdateCluster(old, new)
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

	// Get storage plugin
	storage, err := s.op.GetStorage(cs.Spec.Type)
	if err != nil {
		return err
	}

	// Call plugin
	err = storage.DeleteCluster(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	return nil
}

func (s *StorageClusterOperator) submitNodesFor(cs *spec.StorageCluster) error {
	// Create client
	ns_client := qmclient.NewStorageNodes(s.op.rclient, cs.Namespace)

	// Create a reference object
	clusterRef, err := api.GetReference(cs)
	if err != nil {
		return s.op.logger.Log("err", err)
	}

	// Create nodes
	for _, ns := range cs.Spec.StorageNodes {

		// Get unique deterministic hash for this node
		// since there a maximum Name file
		storageNodeSpecHash := GetStorageNodeSpecHash(ns)

		// Setup StorageNode object
		node := &spec.StorageNode{
			TypeMeta: unversioned.TypeMeta{
				Kind:       "StorageNode",
				APIVersion: cs.APIVersion,
			},
			ObjectMeta: api.ObjectMeta{
				Name:      cs.Name + "-" + fmt.Sprintf("%d", storageNodeSpecHash),
				Namespace: cs.Namespace,
				Labels:    cs.GetLabels(),
			},
			Spec: ns,
		}
		node.Spec.Image = cs.Spec.Image
		node.Spec.ClusterRef = clusterRef
		node.Spec.Type = cs.Spec.Type

		// Submit the node
		result, err := ns_client.Create(node)
		if apierrors.IsAlreadyExists(err) {
			continue
		} else if err != nil {
			return s.op.logger.Log("msg", "unable to create a storage node", "err", err)
		} else {
			s.op.logger.Log("msg", "created a storage node", "name", result.GetName())
		}
	}

	return nil
}

func GetStorageNodeSpecHash(sp spec.StorageNodeSpec) uint32 {
	sphash := adler32.New()
	hashutil.DeepHashObject(sphash, sp)
	return sphash.Sum32()
}

/*
func (s *StorageClusterOperator) listStorageNodes(cs *spec.StorageCluster) ([]*spec.StorageNode, error) {
	storageNodes := qmclient.NewStorageNodes(s.op.rclient, cs.GetNamespace())
	snList, err := storageNodes.List(api.ListOptions{})
	if err != nil {
		return nil, s.op.logger.Log("err", err)
	}

	nodes := make([]*spec.StorageNode, len(snList.Items))
	for _, node := range snList.Items {
		if node.Spec.ClusterRef != nil &&
			node.Spec.ClusterRef.Name == cs.GetName() {
			nodes = append(nodes, &node)
		}
	}

	return nodes, nil
}

func (s *StorageClusterOperator) generateStorageNodeName(cs *spec.StorageCluster, nspec *spec.StorageNodeSpec) string {
	return cs.GetName() + "-" + nspec.NodeName

}
*/
