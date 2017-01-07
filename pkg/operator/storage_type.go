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
)

type StorageClusterInterface interface {
	AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error)
	UpdateCluster(old *spec.StorageCluster, new *spec.StorageCluster) error
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

type StorageTypeNewFunc func(*clientset.Clientset) (StorageType, error)
