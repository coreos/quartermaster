// Copyright 2017 Thiago da Silva
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

package swift

import (
	"github.com/coreos/quartermaster/pkg/spec"
	qmstorage "github.com/coreos/quartermaster/pkg/storage"
	"github.com/heketi/utils"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/util/intstr"
)

var (
	logger = utils.NewLogger("swift", utils.LEVEL_DEBUG)
)

// This mock storage system serves as an example driver for developers
func New(client clientset.Interface, qm restclient.Interface) (qmstorage.StorageType, error) {
	s := &SwiftStorage{
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

type SwiftStorage struct {
	client clientset.Interface
	qm     restclient.Interface
}

func (st *SwiftStorage) Init() error {
	logger.Debug("called")
	return nil
}

func (st *SwiftStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Info("Add cluster %v", c.GetName())
	return nil, nil
}

func (st *SwiftStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Info("Updating cluster %v", old.GetName())
	return nil
}

func (st *SwiftStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Info("Deleting cluster %v", c.GetName())

	return nil
}

func (st *SwiftStorage) MakeDeployment(s *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {

	logger.Debug("Make deployment for node %v", s.GetName())
	if s.Spec.Image == "" {
		s.Spec.Image = "thiagodasilva/swift-saio-poc"
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

func (st *SwiftStorage) makeDeploymentSpec(s *spec.StorageNode) (*extensions.DeploymentSpec, error) {

	spec := &extensions.DeploymentSpec{
		Replicas: 1,
		Template: api.PodTemplateSpec{
			ObjectMeta: api.ObjectMeta{
				Labels: map[string]string{
					// Drivers *should* add a quartermaster label
					"quartermaster": s.Name,
				},
			},
			Spec: api.PodSpec{
				NodeName:     s.Spec.NodeName,
				NodeSelector: s.Spec.NodeSelector,
				Containers: []api.Container{
					api.Container{
						Name:            s.Name,
						Image:           s.Spec.Image,
						ImagePullPolicy: api.PullIfNotPresent,
						Ports: []api.ContainerPort{
							api.ContainerPort{
								ContainerPort: 8080,
							},
						},
					},
				},
			},
		},
	}
	return spec, nil
}

func (st *SwiftStorage) AddNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Info("Adding node %v", s.GetName())

	// Create service to access Swift Proxy API
	err := st.deploySwiftProxyService(s)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (st *SwiftStorage) UpdateNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Info("Updating storage node %v", s.GetName())
	return nil, nil
}

func (st *SwiftStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Info("Deleting storage node %v", s.GetName())
	services := st.client.Core().Services(s.Namespace)
	err := services.Delete("swiftservice", nil)
	if err != nil {
		return err
	}
	return nil
}

func (st *SwiftStorage) Type() spec.StorageTypeIdentifier {

	// This variable must be defined under the spec pkg
	return spec.StorageTypeIdentifierSwift
}

func (st *SwiftStorage) deploySwiftProxyService(sns *spec.StorageNode) error {
	s := &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      "swiftservice",
			Namespace: sns.Namespace,
			Labels: map[string]string{
				"swift": "swift-service",
			},
			Annotations: map[string]string{
				"description": "Exposes Swift Proxy Service",
			},
		},
		Spec: api.ServiceSpec{
			Selector: map[string]string{
				"quartermaster": sns.Name,
			},
			Type: api.ServiceTypeNodePort,
			Ports: []api.ServicePort{
				api.ServicePort{
					Port: 8080,
					TargetPort: intstr.IntOrString{
						IntVal: 8080,
					},
				},
			},
		},
	}

	// Submit the service
	services := st.client.Core().Services(sns.Namespace)
	_, err := services.Create(s)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		logger.Err(err)
	}

	logger.Debug("swift proxy service created")
	return nil
}
