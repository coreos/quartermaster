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

package storage

import (
	"github.com/coreos/quartermaster/pkg/spec"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	restclient "k8s.io/client-go/rest"
)

type StorageClusterInterface interface {
	// AddCluster adds a new storage cluster.
	// AddCluster will be called by Quartermaster controller when a new
	// storage cluster `c` is created as a StorageCluster TPR.
	// AddCluster may save metadata on the StorageCluster resource and
	// return it to Quartermaster for it to submit back to Kubernetes.
	// Quartermaster will create and submit a StorageNode automatically for each
	// described StorageNodeSpec in the StorageCluster, which will in turn create
	// new events.
	AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error)

	// UpdateCluster updates the cluster.
	// UpdateCluster will be called by Quartermaster controller when an existing
	// storage cluster `oldc` is updated to `newc`.
	UpdateCluster(oldc *spec.StorageCluster, newc *spec.StorageCluster) error

	// DeleteCluster deletes the cluster.
	// DeleteCluster will be called by Quartermaster controller when the storage cluster
	// `c` is deleted.  Quartermaster will then automatically go through the list
	// of StorageNodes which were created by this StorageCluster and delete each one.
	// This will cause an event which is then handled by DeleteNode.
	DeleteCluster(c *spec.StorageCluster) error
}

type StorageNodeInterface interface {
	// When a new StorageNode event is received, QM will ask the driver for a
	// Deployment object to deploy the containerized storage software.  This
	// deployment will be submitted to Kubernetes by QM which will then wait
	// until it is ready before calling AddNode().
	//
	// We use Deployments for now because they can handle rollouts and updates
	// well.  We could deploy other object deployment types in the future like
	// (StatefulSets, DaemonSets, etc).
	MakeDeployment(s *spec.StorageNode, old *v1beta1.Deployment) (*v1beta1.Deployment, error)

	// AddNode is called by Quartermaster when a deployment is Ready and is
	// avaiable to be managed by the driver.  AddNode may save metadata on the
	// StorageNode resource and return it to Quartermaster for it to submit
	// back to Kubernetes
	AddNode(s *spec.StorageNode) (*spec.StorageNode, error)

	// UpdateNode is called when Quartermaster is notified of an update to
	// a StorageNode resource
	UpdateNode(s *spec.StorageNode) (*spec.StorageNode, error)

	// DeleteNode is called when Quartermaster is notified of a deletion of
	// a StorageNode resource. This normally happens as a result of a
	// DeleteCluster event.
	DeleteNode(s *spec.StorageNode) error
}

type StorageType interface {
	StorageClusterInterface
	StorageNodeInterface

	// Called on program initialization
	Init() error

	// Must be supplied
	Type() spec.StorageTypeIdentifier
}

// Registers driver with Quartermaster
type StorageTypeNewFunc func(kubernetes.Interface, restclient.Interface) (StorageType, error)
