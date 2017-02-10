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
	"github.com/coreos-inc/quartermaster/pkg/spec"
	qmstorage "github.com/coreos-inc/quartermaster/pkg/storage"
	"github.com/coreos-inc/quartermaster/pkg/utils"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
)

var (
	logger = utils.NewLogger("mock", utils.LEVEL_DEBUG)
)

func New(client clientset.Interface, qm restclient.Interface) (qmstorage.StorageType, error) {
	s := &MockStorage{
		client: client,
		qm:     qm,
	}

	return &qmstorage.StorageHandlerFuncs{
		StorageHandler:     s,
		TypeFunc:           s.Type,
		InitFunc:           s.Init,
		AddClusterFunc:     s.AddCluster,
		UpdateClusterFunc:  s.UpdateCluster,
		DeleteClusterFunc:  s.DeleteCluster,
		MakeDeploymentFunc: s.MakeDeployment,
		AddNodeFunc:        s.AddNode,
		UpdateNodeFunc:     s.UpdateNode,
		DeleteNodeFunc:     s.DeleteNode,
	}, nil
}

type MockStorage struct {
	client clientset.Interface
	qm     restclient.Interface
}

func (st *MockStorage) Init() error {
	logger.Debug("called")
	return nil
}

func (st *MockStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Debug("called")
	return nil, nil
}

func (st *MockStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Debug("called")
	return nil
}

func (st *MockStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Debug("called")
	return nil
}

func (st *MockStorage) MakeDeployment(s *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {

	logger.Debug("Make deployment for node %v", s.GetName())
	if s.Spec.Image == "" {
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
	deployment := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
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

func (st *MockStorage) makeDeploymentSpec(s *spec.StorageNode) (*extensions.DeploymentSpec, error) {

	spec := &extensions.DeploymentSpec{
		Replicas: 1,
		Template: api.PodTemplateSpec{
			ObjectMeta: api.ObjectMeta{
				Labels: map[string]string{
					"nfs-ganesha-node": s.Name,
					"quartermaster":    s.Name,
				},
				Name: s.Name,
			},
			Spec: api.PodSpec{
				NodeName:     s.Spec.NodeName,
				NodeSelector: s.Spec.NodeSelector,
				Containers: []api.Container{
					api.Container{
						Name:            s.Name,
						Image:           s.Spec.Image,
						ImagePullPolicy: api.PullIfNotPresent,
					},
				},
			},
		},
	}
	return spec, nil
}

func (st *MockStorage) AddNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("called")
	return nil, nil
}

func (st *MockStorage) UpdateNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("called")
	return nil, nil
}

func (st *MockStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Debug("called")
	return nil
}

func (st *MockStorage) Type() spec.StorageTypeIdentifier {
	return spec.StorageTypeIdentifierMock
}
