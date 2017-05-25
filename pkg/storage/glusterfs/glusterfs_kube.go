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
	"github.com/coreos/quartermaster/pkg/spec"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	storageclasspkg "k8s.io/client-go/pkg/apis/storage/v1"
)

const (
	glusterfsRoot = "/var/lib/glusterfs-container"
)

func (st *GlusterStorage) makeGlusterFSDeploymentSpec(s *spec.StorageNode) (*v1beta1.DeploymentSpec, error) {
	volumes := []v1.Volume{
		v1.Volume{
			Name: "glusterfs-heketi",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: glusterfsRoot + "/heketi",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-run",
		},
		v1.Volume{
			Name: "glusterfs-lvm",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/run/lvm",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-etc",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: glusterfsRoot + "/etc",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-logs",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: glusterfsRoot + "/logs",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-config",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: glusterfsRoot + "/glusterd",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-dev",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/dev",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-cgroup",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/sys/fs/cgroup",
				},
			},
		},
		v1.Volume{
			Name: "glusterfs-misc",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: glusterfsRoot + "/glusterfsd-misc",
				},
			},
		},
	}

	mounts := []v1.VolumeMount{
		v1.VolumeMount{
			Name:      "glusterfs-heketi",
			MountPath: "/var/lib/heketi",
		},
		v1.VolumeMount{
			Name:      "glusterfs-run",
			MountPath: "/run",
		},
		v1.VolumeMount{
			Name:      "glusterfs-lvm",
			MountPath: "/run/lvm",
		},
		v1.VolumeMount{
			Name:      "glusterfs-etc",
			MountPath: "/etc/glusterfs",
		},
		v1.VolumeMount{
			Name:      "glusterfs-logs",
			MountPath: "/var/log/glusterfs",
		},
		v1.VolumeMount{
			Name:      "glusterfs-config",
			MountPath: "/var/lib/glusterd",
		},
		v1.VolumeMount{
			Name:      "glusterfs-dev",
			MountPath: "/dev",
		},
		v1.VolumeMount{
			Name:      "glusterfs-cgroup",
			MountPath: "/sys/fs/cgroup",
		},
		v1.VolumeMount{
			Name:      "glusterfs-misc",
			MountPath: "/var/lib/misc/glusterfsd",
		},
	}

	probe := &v1.Probe{
		TimeoutSeconds:      3,
		InitialDelaySeconds: 60,
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{
					"/bin/bash",
					"-c",
					"systemctl status glusterd.service",
				},
			},
		},
	}

	priv := true
	replicas := int32(1)
	spec := &v1beta1.DeploymentSpec{
		Replicas: &replicas,
		Template: v1.PodTemplateSpec{
			ObjectMeta: meta.ObjectMeta{
				Labels: map[string]string{
					"quartermaster":  s.Name,
					"name":           "glusterfs",
					"glusterfs":      "pod",
					"glusterfs-node": s.Spec.NodeName,
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
						VolumeMounts:    mounts,
						LivenessProbe:   probe,
						ReadinessProbe:  probe,
						SecurityContext: &v1.SecurityContext{
							Privileged: &priv,
						},
					},
				},
				Volumes:     volumes,
				HostNetwork: true,
			},
		},
	}
	return spec, nil
}

// Deploy a *single* StorageClass for all GlusterFS clusters in this
// namespace.  Even if there are many GlusterFS clusters in a single namespace,
// Heketi takes care of creating a volume from any of them.  Therefore,
// there is a single StorageClass per Heketi instance.
func (st *GlusterStorage) deployStorageClass(namespace string) error {

	// Get Heketi address from service
	heketiAddress, err := st.getHeketiAddress(namespace)
	if err != nil {
		return err
	}

	// Create a name for the storageclass for this namespace
	scname := "gluster.qm." + namespace

	// Create storage class
	storageclass := &storageclasspkg.StorageClass{
		ObjectMeta: meta.ObjectMeta{
			Name:      scname,
			Namespace: namespace,
			Labels: map[string]string{
				"quartermaster": scname,
				"name":          scname,
			},
		},
		Provisioner: "kubernetes.io/glusterfs",
		Parameters: map[string]string{
			"resturl": heketiAddress,
		},
	}

	// Register storage class
	storageclasses := st.client.Storage().StorageClasses()
	_, err = storageclasses.Create(storageclass)
	if apierrors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		logger.Err(err)
	}

	logger.Info("StorageClass registered. Ready for provisioning")

	return nil
}
