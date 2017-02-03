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
	"github.com/coreos-inc/quartermaster/pkg/spec"

	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type StorageClusterInterface interface {
	// AddCluster adds a new storage cluster.
	// AddCluster will be called by Quartermaster controller when a new
	// storage cluster `c` is created as a StorageCluster TPR.
	AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error)

	// UpdateCluster updates the cluster.
	// UpdateCluster will be called by Quartermaster controller when an existing
	// storage cluster `oldc` is updated to `newc`.
	UpdateCluster(oldc *spec.StorageCluster, newc *spec.StorageCluster) error

	// DeleteCluster deletes the cluster.
	// DeleteCluster will be called by Quartermaster controller when the storage cluster
	// `c` is deleted.
	DeleteCluster(c *spec.StorageCluster) error
}

type StorageNodeInterface interface {
	MakeDeployment(c *spec.StorageCluster,
		s *spec.StorageNode,
		old *extensions.Deployment) (*extensions.Deployment, error)
	AddNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error)
	UpdateNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error)
	DeleteNode(s *spec.StorageNode) error
}

type StorageType interface {
	StorageClusterInterface
	StorageNodeInterface

	Init() error
	GetStatus(c *spec.StorageCluster) (*spec.StorageStatus, error)

	// Must be supplied
	Type() spec.StorageTypeIdentifier
}

type StorageTypeNewFunc func(clientset.Interface, restclient.Interface) (StorageType, error)
