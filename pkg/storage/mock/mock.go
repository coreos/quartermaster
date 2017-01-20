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
	"os"
	"strings"

	"github.com/coreos-inc/quartermaster/pkg/spec"
	qmstorage "github.com/coreos-inc/quartermaster/pkg/storage"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/levels"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
)

var (
	logger levels.Levels
)

func init() {
	logger = levels.New(log.NewContext(log.NewLogfmtLogger(os.Stdout)).
		With("ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller))
}

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
		GetStatusFunc:      s.GetStatus,
	}, nil
}

type MockStorage struct {
	client clientset.Interface
	qm     restclient.Interface
}

func (st *MockStorage) Init() error {
	logger.Debug().Log("msg", "init")
	return nil
}

func (st *MockStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Debug().Log("msg", "add cluster", "cluster", c.Name)
	return nil, nil
}

func (st *MockStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Debug().Log("msg", "update", "cluster", old.Name)
	return nil
}

func (st *MockStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Debug().Log("msg", "delete", "cluster", c.Name)
	return nil
}

func (st *MockStorage) MakeDeployment(c *spec.StorageCluster,
	s *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {
	logger.Debug().Log("msg", "make deployment", "node", s.Name)

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
	var volumes []api.Volume
	var mounts []api.VolumeMount

	for _, path := range s.Spec.Directories {
		dash := dashifyPath(path)
		volumes = append(volumes, api.Volume{
			Name: dash,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: path,
				},
			},
		})
		mounts = append(mounts, api.VolumeMount{
			Name:      dash,
			MountPath: path,
		})
	}

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
						VolumeMounts:    mounts,
					},
				},
				Volumes: volumes,
			},
		},
	}
	return spec, nil
}

func (st *MockStorage) AddNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug().Log("msg", "add node", "storagenode", s.Name)
	return nil, nil
}

func (st *MockStorage) UpdateNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug().Log("msg", "update node", "storagenode", s.Name)
	return nil, nil
}

func (st *MockStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Debug().Log("msg", "delete node", "storagenode", s.Name)
	return nil
}

func (st *MockStorage) GetStatus(c *spec.StorageCluster) (*spec.StorageStatus, error) {
	logger.Debug().Log("msg", "status", "cluster", c.Name)
	status := &spec.StorageStatus{}
	return status, nil
}

func (st *MockStorage) Type() spec.StorageTypeIdentifier {
	return spec.StorageTypeIdentifierMock
}

func dashifyPath(s string) string {
	s = strings.TrimLeft(s, "/")
	return strings.Replace(s, "/", "-", -1)
}
