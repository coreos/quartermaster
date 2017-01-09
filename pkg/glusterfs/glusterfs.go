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
	"os"
	"sort"
	"strings"
	"time"

	qmclient "github.com/coreos-inc/quartermaster/pkg/client"
	"github.com/coreos-inc/quartermaster/pkg/operator"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/levels"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/intstr"

	heketiclient "github.com/heketi/heketi/client/api/go-client"
	heketiapi "github.com/heketi/heketi/pkg/glusterfs/api"
)

var (
	logger levels.Levels
)

func init() {
	logger = levels.New(log.NewContext(log.NewLogfmtLogger(os.Stdout)).
		With("ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller))
}

func New(client *clientset.Clientset) (operator.StorageType, error) {
	s := &GlusterStorage{
		client: client,
	}

	return &operator.StorageHandlerFuncs{
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
	client *clientset.Clientset
}

func (st *GlusterStorage) Init() error {
	logger.Debug().Log("msg", "init")
	return nil
}

func (st *GlusterStorage) AddCluster(c *spec.StorageCluster) (*spec.StorageCluster, error) {
	logger.Debug().Log("msg", "add cluster", "cluster", c.Name)

	// Make sure Heketi is up and running
	err := st.deployHeketi(c.Namespace)
	if err != nil {
		return nil, err
	}

	// Get a client and test Hello
	if c.Spec.GlusterFS == nil || len(c.Spec.GlusterFS.Cluster) == 0 {
		httpAddress, err := st.getHeketiAddress(c.GetNamespace())
		if err != nil {
			return nil, err
		}

		h := heketiclient.NewClientNoAuth("http://" + httpAddress + ":8080")
		err = h.Hello()
		if err != nil {
			return nil, logger.Error().Log("err", "unable to communicate with Heketi",
				"address", httpAddress)
		} else {
			logger.Debug().Log("msg", "heketi communication successful")
		}

		// Create a new cluster
		hcluster, err := h.ClusterCreate()
		if err != nil {
			return nil, logger.Error().Log("err", "unable to create cluster in Heketi")
		} else {
			logger.Debug().Log("msg", "Created cluster", "cluster", c.GetName(), "cluster id", hcluster.Id)
		}

		// Save cluster id in the spec
		c.Spec.GlusterFS = &spec.GlusterStorageCluster{
			Cluster: hcluster.Id,
		}

		return c, nil
	}

	// No changes needed
	logger.Info().Log("msg", "cluster already registered",
		"cluster", c.GetName(),
		"cluster id", c.Spec.GlusterFS.Cluster)
	return nil, nil
}

func (st *GlusterStorage) getHeketiAddress(namespace string) (string, error) {
	/*
		service, err := st.client.Core().Services(namespace).Get("heketi")
		if err != nil {
			return "", logger.Error().Log("msg", "error accessing heketi service", "err", err)
		}
	*/

	//return service.Spec.ClusterIP, nil
	// During development, running QM remotely, setup a portforward to heketi pod
	return "localhost", nil
}

func (st *GlusterStorage) UpdateCluster(old *spec.StorageCluster,
	new *spec.StorageCluster) error {
	logger.Debug().Log("msg", "update", "cluster", old.Name)
	return nil
}

func (st *GlusterStorage) DeleteCluster(c *spec.StorageCluster) error {
	logger.Debug().Log("msg", "delete", "cluster", c.Name)
	return nil
}

func (st *GlusterStorage) MakeDeployment(c *spec.StorageCluster,
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

func (st *GlusterStorage) makeDeploymentSpec(s *spec.StorageNode) (*extensions.DeploymentSpec, error) {
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
	logger.Debug().Log("msg", "add node", "storagenode", s.Name)

	httpAddress, err := st.getHeketiAddress(s.GetNamespace())
	if err != nil {
		return nil, err
	}

	h := heketiclient.NewClientNoAuth("http://" + httpAddress + ":8080")
	err = h.Hello()
	if err != nil {
		return nil, logger.Error().Log("err", "unable to communicate with Heketi",
			"address", httpAddress)
	}

	// Update cluster
	clusters := qmclient.NewStorageClusters(st.qm, s.GetNamespace())
	cluster, err := clusters.Get(s.Spec.ClusterRef.Name)
	if err != nil {
		return nil, logger.Error().Log("err", err)
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
		return nil, logger.Error().Log("msg", "unable to add node", "node", s.GetName(), "err", err)
	}

	// Update node with new information
	s.Spec.GlusterFS.Cluster = cluster.Spec.GlusterFS.Cluster
	s.Spec.GlusterFS.Node = node.Id
	s.Status.Ready = true

	return s, nil
}

func (st *GlusterStorage) UpdateNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug().Log("msg", "update node", "storagenode", s.Name)
	return nil, nil
}

func (st *GlusterStorage) DeleteNode(s *spec.StorageNode) error {
	logger.Debug().Log("msg", "delete node", "storagenode", s.Name)

	if len(s.Spec.GlusterFS.Node) == 0 {
		return nil
	}

	httpAddress, err := st.getHeketiAddress(s.GetNamespace())
	if err != nil {
		return err
	}

	h := heketiclient.NewClientNoAuth("http://" + httpAddress + ":8080")
	err = h.Hello()
	if err != nil {
		return logger.Error().Log("err", "unable to communicate with Heketi",
			"address", httpAddress)
	}

	return h.NodeDelete(s.Spec.GlusterFS.Node)
}

func (st *GlusterStorage) GetStatus(c *spec.StorageCluster) (*spec.StorageStatus, error) {
	logger.Debug().Log("msg", "status", "cluster", c.Name)
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
		logger.Error().Log("err", err)
	}

	// REMOVE THIS
	// TODO(lpabon) replace with actual wait()
	time.Sleep(time.Second * 30)

	logger.Debug().Log("msg", "heketi deployed")
	return nil
}

func (st *GlusterStorage) deployHeketiServiceAccount(namespace string) error {
	s := &api.ServiceAccount{
		ObjectMeta: api.ObjectMeta{
			Name: "heketi-service-account",
		},
	}

	// Submit the service account
	serviceaccounts := st.client.Core().ServiceAccounts(namespace)
	_, err := serviceaccounts.Create(s)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		logger.Error().Log("err", err)
	}

	logger.Debug().Log("msg", "service account created")
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
		logger.Error().Log("err", err)
	}

	logger.Debug().Log("msg", "service account created")
	return nil
}
