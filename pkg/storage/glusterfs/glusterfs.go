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

package glusterfs

import (
	"sort"
	"strings"
	"time"

	qmclient "github.com/coreos-inc/quartermaster/pkg/client"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	qmstorage "github.com/coreos-inc/quartermaster/pkg/storage"
	"github.com/coreos-inc/quartermaster/pkg/utils"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/util/intstr"

	heketiclient "github.com/heketi/heketi/client/api/go-client"
	heketiapi "github.com/heketi/heketi/pkg/glusterfs/api"
)

var (
	logger          = utils.NewLogger("glusterfs", utils.LEVEL_DEBUG)
	heketiAddressFn = func(namespace string) (string, error) {
		return "http://localhost:8080", nil
	}
)

func New(client clientset.Interface, qm restclient.Interface) (qmstorage.StorageType, error) {
	s := &GlusterStorage{
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

type GlusterStorage struct {
	client clientset.Interface
	qm     restclient.Interface
}

func (st *GlusterStorage) Init() error {
	logger.Debug("msg init")
	return nil
}

func (st *GlusterStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Debug("msg add cluster cluster %v", c.Name)

	// Make sure Heketi is up and running
	err := st.deployHeketi(c.Namespace)
	if err != nil {
		return nil, err
	}

	// Get a client
	if c.Spec.GlusterFS == nil || len(c.Spec.GlusterFS.Cluster) == 0 {
		httpAddress, err := heketiAddressFn(c.GetNamespace())
		if err != nil {
			return nil, err
		}

		// Create a new cluster
		// TODO(lpabon): Need to set user and secret
		h := heketiclient.NewClientNoAuth(httpAddress)
		hcluster, err := h.ClusterCreate()
		if err != nil {
			return nil, logger.LogError("err: unable to create cluster in Heketi: %v")
		} else {
			logger.Debug("Created cluster %v cluster id %v", c.GetName(), hcluster.Id)
		}

		// Save cluster id in the spec
		c.Spec.GlusterFS = &spec.GlusterStorageCluster{
			Cluster: hcluster.Id,
		}

		return c, nil
	}

	// No changes needed
	logger.Info("cluster already registered: cluster[%v] id[%v]",
		c.GetName(),
		c.Spec.GlusterFS.Cluster)
	return nil, nil
}

func (st *GlusterStorage) getHeketiAddress(namespace string) (string, error) {
	/*
		service, err := st.client.Core().Services(namespace).Get("heketi")
		if err != nil {
			return "", logger.LogError().Log("msg", "error accessing heketi service", "err", err)
		}
	*/

	//return service.Spec.ClusterIP, nil
	// During development, running QM remotely, setup a portforward to heketi pod
	return "http://localhost:8080", nil
}

func (st *GlusterStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Debug("update cluster %v", old.Name)
	return nil
}

func (st *GlusterStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Debug("delete cluster %v", c.Name)
	return nil
}

func (st *GlusterStorage) MakeDeployment(c *spec.StorageCluster,
	s *spec.StorageNode,
	old *extensions.Deployment) (*extensions.Deployment, error) {
	logger.Debug("make deployment node %v", s.Name)

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

func (st *GlusterStorage) makeDeploymentSpec(s *spec.StorageNode) (*extensions.DeploymentSpec, error) {
	var volumes []api.Volume
	var mounts []api.VolumeMount

	// TODO(lpabon): Remove this
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
					"quartermaster": s.Name,
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

func (st *GlusterStorage) AddNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("add node storagenode %v", s.Name)

	httpAddress, err := heketiAddressFn(s.GetNamespace())
	if err != nil {
		return nil, err
	}

	h := heketiclient.NewClientNoAuth("http://" + httpAddress + ":8080")
	// Update cluster
	clusters := qmclient.NewStorageClusters(st.qm, s.GetNamespace())
	cluster, err := clusters.Get(s.Spec.ClusterRef.Name)
	if err != nil {
		return nil, logger.Err(err)
	}

	// Add node to Heketi
	nodereq := &heketiapi.NodeAddRequest{
		Zone:      s.Spec.GlusterFS.Zone,
		ClusterId: cluster.Spec.GlusterFS.Cluster,
		Hostnames: heketiapi.HostAddresses{
			Manage: sort.StringSlice{s.Spec.NodeName},

			// TODO(lpabon): Real IP of node must be added here
			Storage: sort.StringSlice{s.Spec.NodeName},
		},
	}
	node, err := h.NodeAdd(nodereq)
	if err != nil {
		return nil, logger.LogError("unable to add node %v: %v", s.GetName(), err)
	}

	// Update node with new information
	s.Spec.GlusterFS.Cluster = cluster.Spec.GlusterFS.Cluster
	s.Spec.GlusterFS.Node = node.Id
	s.Status.Ready = true

	return s, nil
}

func (st *GlusterStorage) UpdateNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug("update node storagenode %v", s.Name)
	return nil, nil
}

func (st *GlusterStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Debug("delete node storagenode %v", s.Name)

	if len(s.Spec.GlusterFS.Node) == 0 {
		return nil
	}

	httpAddress, err := heketiAddressFn(s.GetNamespace())
	if err != nil {
		return err
	}

	h := heketiclient.NewClientNoAuth("http://" + httpAddress + ":8080")
	return h.NodeDelete(s.Spec.GlusterFS.Node)
}

func (st *GlusterStorage) GetStatus(c *spec.StorageCluster) (*spec.StorageStatus, error) {
	logger.Debug("status")
	status := &spec.StorageStatus{}
	return status, nil
}

func (st *GlusterStorage) Type() spec.StorageTypeIdentifier {
	return spec.StorageTypeIdentifierGlusterFS
}

func dashifyPath(s string) string {
	s = strings.TrimLeft(s, "/")
	return strings.Replace(s, "/", "-", -1)
}

func (st *GlusterStorage) deployHeketi(namespace string) error {
	// Create a service account for Heketi
	err := st.deployHeketiServiceAccount(namespace)
	if err != nil {
		return err
	}

	// Deployment
	err = st.deployHeketiPod(namespace)
	if err != nil {
		return err
	}

	// Create service to access Heketi API
	return st.deployHeketiService(namespace)
}

func (st *GlusterStorage) deployHeketiPod(namespace string) error {

	// Deployment for Heketi
	d := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name:      "heketi",
			Namespace: namespace,
			Annotations: map[string]string{
				"description": "Defines how to deploy Heketi",
			},
			Labels: map[string]string{
				"glusterfs":     "heketi-deployment",
				"heketi":        "heketi-deployment",
				"quartermaster": "heketi",
			},
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"glusterfs": "heketi-deployment",
						"heketi":        "heketi-deployment",
						"quartermaster": "heketi",
						"name":          "heketi",
					},
					Name: "heketi",
				},
				Spec: api.PodSpec{
					ServiceAccountName: "heketi-service-account",
					Containers: []api.Container{
						api.Container{
							Name:            "heketi",
							Image:           "heketi/heketi:dev",
							ImagePullPolicy: api.PullIfNotPresent,
							Env: []api.EnvVar{
								api.EnvVar{
									Name: "HEKETI_EXECUTOR",

									// TODO(lpabon): DEMO ONLY.  Put back
									// to a value of "kubernetes"
									Value: "mock",
								},
								api.EnvVar{
									Name:  "HEKETI_FSTAB",
									Value: "/var/lib/heketi/fstab",
								},
								api.EnvVar{
									Name:  "HEKETI_SNAPSHOT_LIMIT",
									Value: "14",
								},
							},
							Ports: []api.ContainerPort{
								api.ContainerPort{
									ContainerPort: 8080,
								},
							},
							VolumeMounts: []api.VolumeMount{
								api.VolumeMount{
									Name:      "db",
									MountPath: "/var/lib/heketi",
								},
							},
							ReadinessProbe: &api.Probe{
								TimeoutSeconds:      3,
								InitialDelaySeconds: 3,
								Handler: api.Handler{
									HTTPGet: &api.HTTPGetAction{
										Path: "/hello",
										Port: intstr.IntOrString{
											IntVal: 8080,
										},
									},
								},
							},
							LivenessProbe: &api.Probe{
								TimeoutSeconds:      3,
								InitialDelaySeconds: 30,
								Handler: api.Handler{
									HTTPGet: &api.HTTPGetAction{
										Path: "/hello",
										Port: intstr.IntOrString{
											IntVal: 8080,
										},
									},
								},
							},
						},
					},
					Volumes: []api.Volume{
						api.Volume{
							Name: "db",
						},
					},
				},
			},
		},
	}

	deployments := st.client.Extensions().Deployments(namespace)
	_, err := deployments.Create(d)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		logger.Err(err)
	}

	// REMOVE THIS
	// TODO(lpabon) replace with actual wait()
	time.Sleep(time.Microsecond * 30)

	logger.Debug("heketi deployed")
	return nil
}

func (st *GlusterStorage) deployHeketiServiceAccount(namespace string) error {
	s := &api.ServiceAccount{
		ObjectMeta: api.ObjectMeta{
			Name:      "heketi-service-account",
			Namespace: namespace,
		},
	}

	// Submit the service account
	serviceaccounts := st.client.Core().ServiceAccounts(namespace)
	_, err := serviceaccounts.Create(s)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		return logger.Err(err)
	}

	logger.Debug("service account created")
	return nil
}

func (st *GlusterStorage) deployHeketiService(namespace string) error {
	s := &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      "heketi",
			Namespace: namespace,
			Labels: map[string]string{
				"glusterfs": "heketi-service",
				"heketi":    "support",
			},
			Annotations: map[string]string{
				"description": "Exposes Heketi Service",
			},
		},
		Spec: api.ServiceSpec{
			Selector: map[string]string{
				"name": "heketi",
			},
			Ports: []api.ServicePort{
				api.ServicePort{
					Name: "heketi",
					Port: 8080,
					TargetPort: intstr.IntOrString{
						IntVal: 8080,
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
