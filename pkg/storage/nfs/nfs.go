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

package nfs

import (
	"fmt"
	"strings"

	qmclient "github.com/coreos/quartermaster/pkg/client"
	"github.com/coreos/quartermaster/pkg/spec"
	qmstorage "github.com/coreos/quartermaster/pkg/storage"
	"github.com/heketi/utils"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/util/intstr"
)

var (
	logger = utils.NewLogger("nfs", utils.LEVEL_DEBUG)
)

type NfsStorage struct {
	client clientset.Interface
	qm     restclient.Interface
}

func New(client clientset.Interface, qm restclient.Interface) (qmstorage.StorageType, error) {
	s := &NfsStorage{
		client: client,
		qm:     qm,
	}

	return &qmstorage.StorageHandlerFuncs{
		StorageHandler:     s,
		TypeFunc:           s.Type,
		InitFunc:           s.Init,
		MakeDeploymentFunc: s.MakeDeployment,
		AddNodeFunc:        s.AddNode,
		UpdateNodeFunc:     s.UpdateNode,
		DeleteNodeFunc:     s.DeleteNode,
	}, nil
}

func (st *NfsStorage) Init() error {
	// Nothing to initialize, no external managing containers to check, very simple.
	return nil
}

func (st *NfsStorage) MakeDeployment(s *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {

	if s.Spec.Image == "" {
		s.Spec.Image = "quay.io/luis_pabon0/ganesha:latest"
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

func dashifyPath(s string) string {
	s = strings.TrimLeft(s, "/")
	return strings.Replace(s, "/", "-", -1)
}

func (st *NfsStorage) makeDeploymentSpec(s *spec.StorageNode) (*extensions.DeploymentSpec, error) {
	if len(s.Spec.Devices) != 0 {
		return nil, fmt.Errorf("NFS does not support raw device access")
	}
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

	privileged := true

	spec := &extensions.DeploymentSpec{
		Replicas: 1,
		Template: api.PodTemplateSpec{
			ObjectMeta: api.ObjectMeta{
				Labels: map[string]string{
					"name":             s.Name,
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
						SecurityContext: &api.SecurityContext{
							Privileged: &privileged,
						},

						Ports: []api.ContainerPort{
							api.ContainerPort{
								Name:          "nfs",
								ContainerPort: 2049,
								//TODO(barakmich)
								// HostIP: <get IP from spec>
							},
							api.ContainerPort{
								Name:          "mountd",
								ContainerPort: 20048,
								//TODO(barakmich)
								// HostIP: <get IP from spec>
							},
							api.ContainerPort{
								Name:          "rpcbind",
								ContainerPort: 111,
								//TODO(barakmich)
								// HostIP: <get IP from spec>
							},
						},
					},
				},
				Volumes: volumes,
			},
		},
	}
	return spec, nil
}

func (st *NfsStorage) AddNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("add node %v", s.GetName())

	// Create Service for this node
	err := st.deployNfsService(s.GetNamespace(), s.GetName())
	if err != nil {
		return nil, logger.LogError("Failed to deploy service for %v/%v: %v",
			s.GetNamespace(), s.GetName(), err)
	}

	// Create PV
	err = st.deployPv(s)
	if err != nil {
		return nil, logger.LogError("Failed to deploy pv %s/%s: %v",
			s.GetNamespace(), s.GetName(), err)
	}

	// Update status of node and cluster
	s.Status.Ready = true
	s.Status.Message = "NFS Started"
	s.Status.Reason = "Success"

	// Update cluster
	clusters := qmclient.NewStorageClusters(st.qm, s.GetNamespace())
	cluster, err := clusters.Get(s.Spec.ClusterRef.Name)
	if err != nil {
		return nil, logger.Err(err)
	}

	cluster.Status.Ready = true
	cluster.Status.Message = "NFS started by storagenode " + s.GetName()
	cluster.Status.Reason = "Success"
	_, err = clusters.Update(cluster)
	if err != nil {
		return nil, logger.Err(err)
	}

	return s, nil
}

func (st *NfsStorage) UpdateNode(s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("Update node %v", s.GetName())
	return nil, nil
}

func (st *NfsStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Debug("Delete node %v", s.GetName())

	// Delete PV
	err := st.client.Core().PersistentVolumes().Delete(s.GetName(), nil)
	if err != nil {
		return logger.Err(err)
	}

	// Delete service
	err = st.client.Core().Services(s.GetNamespace()).Delete(s.GetName(), nil)
	if err != nil {
		return logger.Err(err)
	}

	return nil
}

func (st *NfsStorage) Type() spec.StorageTypeIdentifier {
	return spec.StorageTypeIdentifierNFS
}

func (st *NfsStorage) deployPv(s *spec.StorageNode) error {

	// Get IP to service
	service, err := st.client.Core().Services(s.GetNamespace()).Get(s.GetName())
	if err != nil {
		return logger.LogError("Failed to get network address from service %v/%v: %v",
			s.GetNamespace(), s.GetName(), err)
	}

	if len(service.Spec.ClusterIP) == 0 {
		return logger.LogError("Service %v/%v does not contain a cluster IP",
			s.GetNamespace(), s.GetName())
	}

	// Set values from storagenode spec
	size := "10Gi"
	readOnly := false
	if s.Spec.NFS != nil {
		if len(s.Spec.NFS.Size) != 0 {
			size = s.Spec.NFS.Size
		}
		readOnly = s.Spec.NFS.ReadOnly
	}

	// Create persistent volume
	pv := &api.PersistentVolume{
		ObjectMeta: api.ObjectMeta{
			Name:      s.GetName(),
			Namespace: s.GetNamespace(),
			Annotations: map[string]string{
				"description": "Exposes NFS Service",
			},
		},
		Spec: api.PersistentVolumeSpec{
			Capacity: api.ResourceList{
				api.ResourceName(api.ResourceStorage): resource.MustParse(size),
			},
			AccessModes: []api.PersistentVolumeAccessMode{
				api.ReadWriteMany,
			},
			PersistentVolumeSource: api.PersistentVolumeSource{
				NFS: &api.NFSVolumeSource{
					Server:   service.Spec.ClusterIP,
					ReadOnly: readOnly,

					// TODO(lpabon): This has to be changed when
					// configMaps are supported
					Path: "/exports",
				},
			},
		},
	}

	pvs := st.client.Core().PersistentVolumes()
	_, err = pvs.Create(pv)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		logger.Err(err)
	}

	return nil
}

func (st *NfsStorage) deployNfsService(namespace, name string) error {
	s := &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"description": "Exposes NFS Service",
			},
		},
		Spec: api.ServiceSpec{
			Selector: map[string]string{
				"name": name,
			},
			Ports: []api.ServicePort{
				api.ServicePort{
					Name: "nfs",
					Port: 2049,
					TargetPort: intstr.IntOrString{
						IntVal: 2049,
					},
				},
				api.ServicePort{
					Name: "mountd",
					Port: 20048,
					TargetPort: intstr.IntOrString{
						IntVal: 20048,
					},
				},
				api.ServicePort{
					Name: "rpcbind",
					Port: 111,
					TargetPort: intstr.IntOrString{
						IntVal: 111,
					},
				},
			},
		},
	}

	// Submit the service
	services := st.client.Core().Services(namespace)
	_, err := services.Create(s)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		logger.Err(err)
	}

	logger.Debug("service account created")
	return nil
}
