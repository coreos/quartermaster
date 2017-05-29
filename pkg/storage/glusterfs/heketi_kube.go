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
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"

	heketiclient "github.com/heketi/heketi/client/api/go-client"
)

func (st *GlusterStorage) heketiClient(namespace string) (*heketiclient.Client, error) {
	httpAddress, err := st.getHeketiAddress(namespace)
	if err != nil {
		return nil, err
	}

	// Create a new cluster
	// TODO(lpabon): Need to set user and secret
	return heketiclient.NewClientNoAuth(httpAddress), nil
}

func (st *GlusterStorage) getHeketiAddress(namespace string) (string, error) {
	service, err := st.client.Core().Services(namespace).Get("heketi", meta.GetOptions{})
	if err != nil {
		return "", logger.LogError("Failed to get cluster ip from Heketi service: %v", err)
	}
	if len(service.Spec.Ports) == 0 {
		return "", logger.LogError("Heketi service in namespace %s is missing port value", namespace)
	}
	address := fmt.Sprintf("http://%s:%d",
		service.Spec.ClusterIP,
		service.Spec.Ports[0].Port)
	logger.Debug("Got: %s", address)

	return address, nil
	// During development, running QM remotely, setup a portforward to heketi pod
	//return "http://localhost:8080", nil
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
	replicas := int32(1)
	d := &v1beta1.Deployment{
		ObjectMeta: meta.ObjectMeta{
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
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Labels: map[string]string{"glusterfs": "heketi-deployment",
						"heketi":        "heketi-deployment",
						"quartermaster": "heketi",
						"name":          "heketi",
					},
					Name: "heketi",
				},
				Spec: v1.PodSpec{
					ServiceAccountName: "heketi-service-account",
					Containers: []v1.Container{
						v1.Container{
							Name:            "heketi",
							Image:           "lpabon/heketi:dev",
							ImagePullPolicy: v1.PullIfNotPresent,
							Env: []v1.EnvVar{
								v1.EnvVar{
									Name:  "HEKETI_EXECUTOR",
									Value: "kubernetes",
								},
								v1.EnvVar{
									Name:  "HEKETI_FSTAB",
									Value: "/var/lib/heketi/fstab",
								},
								v1.EnvVar{
									Name:  "HEKETI_SNAPSHOT_LIMIT",
									Value: "14",
								},
								v1.EnvVar{
									Name:  "HEKETI_BACKUP_DB_TO_KUBE_SECRET",
									Value: "yes",
								},
							},
							Ports: []v1.ContainerPort{
								v1.ContainerPort{
									ContainerPort: 8080,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								v1.VolumeMount{
									Name:      "db",
									MountPath: "/var/lib/heketi",
								},
							},
							ReadinessProbe: &v1.Probe{
								TimeoutSeconds:      3,
								InitialDelaySeconds: 3,
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/hello",
										Port: intstr.IntOrString{
											IntVal: 8080,
										},
									},
								},
							},
							LivenessProbe: &v1.Probe{
								TimeoutSeconds:      3,
								InitialDelaySeconds: 30,
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/hello",
										Port: intstr.IntOrString{
											IntVal: 8080,
										},
									},
								},
							},
						},
					},
					Volumes: []v1.Volume{
						v1.Volume{
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

	// Wait until deployment ready
	err = waitForDeploymentFn(st.client, namespace, d.GetName(), *d.Spec.Replicas)
	if err != nil {
		return logger.Err(err)
	}

	logger.Debug("heketi deployed")
	return nil
}

func (st *GlusterStorage) deployHeketiServiceAccount(namespace string) error {
	s := &v1.ServiceAccount{
		ObjectMeta: meta.ObjectMeta{
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
	s := &v1.Service{
		ObjectMeta: meta.ObjectMeta{
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
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"name": "heketi",
			},
			Ports: []v1.ServicePort{
				v1.ServicePort{
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
