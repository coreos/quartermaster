package nfs

import (
	"fmt"
	"strings"

	"github.com/coreos-inc/quartermaster/pkg/operator"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func New(client *kubernetes.Clientset) (operator.StorageType, error) {
	return &Storage{
		client: client,
	}, nil
}

type Storage struct {
	client *kubernetes.Clientset
}

var _ operator.StorageType = &Storage{}

func (st *Storage) Init() error {
	// Nothing to initialize, no external managing containers to check, very simple.
	return nil
}

func (st *Storage) MakeDaemonSet(s *spec.StorageNode, old *v1beta1.DaemonSet) (*v1beta1.DaemonSet, error) {
	if s.Spec.Image == "" {
		s.Spec.Image = "quay.io/luis_pabon0/ganesha:latest"
	}
	spec, err := st.makeDaemonSetSpec(s)
	if err != nil {
		return nil, err
	}
	lmap := make(map[string]string)
	for k, v := range s.Labels {
		lmap[k] = v
	}
	lmap["quartermaster"] = s.Name
	ds := &v1beta1.DaemonSet{
		ObjectMeta: v1.ObjectMeta{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Annotations: s.Annotations,
			Labels:      lmap,
		},
		Spec: *spec,
	}
	if old != nil {
		ds.Annotations = old.Annotations
	}
	return ds, nil
}

func dashifyPath(s string) string {
	s = strings.TrimLeft(s, "/")
	return strings.Replace(s, "/", "-", -1)
}

func (st *Storage) makeDaemonSetSpec(s *spec.StorageNode) (*v1beta1.DaemonSetSpec, error) {
	if len(s.Spec.Devices) != 0 {
		return nil, fmt.Errorf("NFS does not support raw device access")
	}
	var volumes []v1.Volume
	var mounts []v1.VolumeMount

	for _, path := range s.Spec.Directories {
		dash := dashifyPath(path)
		volumes = append(volumes, v1.Volume{
			Name: dash,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
		})
		mounts = append(mounts, v1.VolumeMount{
			Name:      dash,
			MountPath: path,
		})
	}

	privileged := true

	spec := &v1beta1.DaemonSetSpec{
		Template: v1.PodTemplateSpec{
			ObjectMeta: v1.ObjectMeta{},
			Spec: v1.PodSpec{
				NodeSelector: s.Spec.NodeSelector,
				Containers: []v1.Container{
					v1.Container{
						Name:            s.Name,
						Image:           s.Spec.Image,
						ImagePullPolicy: v1.PullIfNotPresent,
						VolumeMounts:    mounts,
						SecurityContext: &v1.SecurityContext{
							Privileged: &privileged,
						},

						Ports: []v1.ContainerPort{
							v1.ContainerPort{
								Name:          "nfs",
								ContainerPort: 2049,
								//TODO(barakmich)
								// HostIP: <get IP from spec>
							},
							v1.ContainerPort{
								Name:          "mountd",
								ContainerPort: 20048,
								//TODO(barakmich)
								// HostIP: <get IP from spec>
							},
							v1.ContainerPort{
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

func (st *Storage) AddNode(s *spec.StorageNode) error {
	return fmt.Errorf("Trying to add another node to an NFS storage set '%s'. NFS is not distributed.", s.Name)
}

func (st *Storage) GetStatus(s *spec.StorageNode) (*spec.StorageStatus, error) {
	status := &spec.StorageStatus{}
	return status, nil
}
