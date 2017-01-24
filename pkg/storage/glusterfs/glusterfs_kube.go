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
	"github.com/coreos-inc/quartermaster/pkg/spec"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

const (
	glusterfsRoot = "/var/lib/glusterfs-container"
)

func (st *GlusterStorage) makeGlusterFSDeploymentSpec(s *spec.StorageNode) (*extensions.DeploymentSpec, error) {
	volumes := []api.Volume{
		api.Volume{
			Name: "glusterfs-heketi",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: glusterfsRoot + "/heketi",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-run",
		},
		api.Volume{
			Name: "glusterfs-lvm",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: "/run/lvm",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-etc",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: glusterfsRoot + "/etc",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-logs",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: glusterfsRoot + "/logs",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-config",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: glusterfsRoot + "/glusterd",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-dev",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: "/dev",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-cgroup",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: "/sys/fs/cgroup",
				},
			},
		},
		api.Volume{
			Name: "glusterfs-misc",
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: glusterfsRoot + "/glusterfsd-misc",
				},
			},
		},
	}

	mounts := []api.VolumeMount{
		api.VolumeMount{
			Name:      "glusterfs-heketi",
			MountPath: "/var/lib/heketi",
		},
		api.VolumeMount{
			Name:      "glusterfs-run",
			MountPath: "/run",
		},
		api.VolumeMount{
			Name:      "glusterfs-lvm",
			MountPath: "/run/lvm",
		},
		api.VolumeMount{
			Name:      "glusterfs-etc",
			MountPath: "/etc/glusterfs",
		},
		api.VolumeMount{
			Name:      "glusterfs-logs",
			MountPath: "/var/log/glusterfs",
		},
		api.VolumeMount{
			Name:      "glusterfs-config",
			MountPath: "/var/lib/glusterd",
		},
		api.VolumeMount{
			Name:      "glusterfs-dev",
			MountPath: "/dev",
		},
		api.VolumeMount{
			Name:      "glusterfs-cgroup",
			MountPath: "/sys/fs/cgroup",
		},
		api.VolumeMount{
			Name:      "glusterfs-misc",
			MountPath: "/var/lib/misc/glusterfsd",
		},
	}

	probe := &api.Probe{
		TimeoutSeconds:      3,
		InitialDelaySeconds: 60,
		Handler: api.Handler{
			Exec: &api.ExecAction{
				Command: []string{
					"/bin/bash",
					"-c",
					"systemctl status glusterd.service",
				},
			},
		},
	}

	priv := true
	spec := &extensions.DeploymentSpec{
		Replicas: 1,
		Template: api.PodTemplateSpec{
			ObjectMeta: api.ObjectMeta{
				Labels: map[string]string{
					"quartermaster":  s.Name,
					"name":           "glusterfs",
					"glusterfs":      "pod",
					"glusterfs-node": s.Spec.NodeName,
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
						LivenessProbe:   probe,
						ReadinessProbe:  probe,
						SecurityContext: &api.SecurityContext{
							Privileged: &priv,
						},
					},
				},
				Volumes: volumes,
				SecurityContext: &api.PodSecurityContext{
					HostNetwork: true,
				},
			},
		},
	}
	return spec, nil
}
