// Copyright 2017 The quartermaster Authors
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

package mock

import (
	"github.com/coreos/quartermaster/pkg/spec"
	qmstorage "github.com/coreos/quartermaster/pkg/storage"
	"github.com/heketi/utils"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	restclient "k8s.io/client-go/rest"
)

var (
	logger = utils.NewLogger("mock", utils.LEVEL_DEBUG)
)

// This mock storage system serves as an example driver for developers
func New(client kubernetes.Interface, qm restclient.Interface) (qmstorage.StorageType, error) {
	s := &MockStorage{
		client: client,
		qm:     qm,
	}

	// Use the StorageHandlerFuncs struct to insulate
	// the driver from interface incompabilities
	return &qmstorage.StorageHandlerFuncs{

		// Save the storage handler.  Great for unit tests
		StorageHandler: s,

		// Provide a function which returns the Type of storage system
		// Required
		TypeFunc: s.Type,

		// This function is called when QM is started
		// Optional
		InitFunc: s.Init,

		// ---------------- Cluster Functions ---------------

		// Called after a new StorageCluster object has been submitted
		// QM takes care of creating StorageNodes for each one defined
		// in the StorageCluster so there is no need for the driver to
		// create them
		AddClusterFunc: s.AddCluster,

		// Called when the StorageCluster has been updated
		UpdateClusterFunc: s.UpdateCluster,

		// Called when the StorageCluster has been deleted. After this
		// call, QM will delete all StorageNode objects.  You may want
		// to wait until all your StorageNodes are deleted, but it all
		// depends on your storage system
		DeleteClusterFunc: s.DeleteCluster,

		// ---------------- Node Functions ---------------

		// Return a Deployment object which requests the installation
		// of the containerized storage software.
		MakeDeploymentFunc: s.MakeDeployment,

		// New StorageNode is ready and it is called after the
		// deployment is available and running.
		AddNodeFunc: s.AddNode,

		// Called when a StorageNode has been updated
		UpdateNodeFunc: s.UpdateNode,

		// Called when a StorageNode has been deleted
		DeleteNodeFunc: s.DeleteNode,
	}, nil
}

type MockStorage struct {
	client kubernetes.Interface
	qm     restclient.Interface
}

func (st *MockStorage) Init() error {
	logger.Debug("called")
	return nil
}

func (st *MockStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Info("Add cluster %v", c.GetName())
	return nil, nil
}

func (st *MockStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Info("Updating cluster %v", old.GetName())
	return nil
}

func (st *MockStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Info("Deleting cluster %v", c.GetName())
	return nil
}

func (st *MockStorage) MakeDeployment(s *spec.StorageNode,
	old *v1beta1.Deployment) (*v1beta1.Deployment, error) {

	logger.Debug("Make deployment for node %v", s.GetName())
	if s.Spec.Image == "" {
		// Use nginx as a 'mock' storage system container
		s.Spec.Image = "nginx"
	}
	spec, err := st.makeDeploymentSpec(s)
	if err != nil {
		return nil, err
	}
	lmap := make(map[string]string)
	for k, v := range s.Labels {
		lmap[k] = v
	}
	lmap["quartermaster"] = s.Name
	deployment := &v1beta1.Deployment{
		ObjectMeta: meta.ObjectMeta{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Annotations: s.Annotations,
			Labels:      lmap,
		},
		Spec: *spec,
	}
	if old != nil {
		deployment.Annotations = old.Annotations
	}
	return deployment, nil
}

func (st *MockStorage) makeDeploymentSpec(s *spec.StorageNode) (*v1beta1.DeploymentSpec, error) {

	replicas := int32(1)
	spec := &v1beta1.DeploymentSpec{
		Replicas: &replicas,
		Template: v1.PodTemplateSpec{
			ObjectMeta: meta.ObjectMeta{
				Labels: map[string]string{
					"nfs-ganesha-node": s.Name,

					// Drivers *should* add a quartermaster label
					"quartermaster": s.Name,
				},
				Name: s.Name,
			},
			Spec: v1.PodSpec{
				NodeName:     s.Spec.NodeName,
				NodeSelector: s.Spec.NodeSelector,
				Containers: []v1.Container{
					v1.Container{
						Name:            s.Name,
						Image:           s.Spec.Image,
						ImagePullPolicy: v1.PullIfNotPresent,
					},
				},
			},
		},
	}
	return spec, nil
}

func (st *MockStorage) AddNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Info("Adding node %v", s.GetName())
	return nil, nil
}

func (st *MockStorage) UpdateNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Info("Updating storage node %v", s.GetName())
	return nil, nil
}

func (st *MockStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Info("Deleting storage node %v", s.GetName())
	return nil
}

func (st *MockStorage) Type() spec.StorageTypeIdentifier {

	// This variable must be defined under the spec pkg
	return spec.StorageTypeIdentifierMock
}
