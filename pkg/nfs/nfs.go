package nfs

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos-inc/quartermaster/pkg/operator"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/levels"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

var (
	logger levels.Levels
)

func init() {
	logger = levels.New(log.NewContext(log.NewLogfmtLogger(os.Stdout)).
		With("ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller))
}

func New(client *clientset.Clientset) (operator.StorageType, error) {
	s := &NfsStorage{
		client: client,
	}

	return &operator.StorageHandlerFuncs{
		StorageHandler:     s,
		TypeFunc:           s.Type,
		InitFunc:           s.Init,
		MakeDeploymentFunc: s.MakeDeployment,
		AddNodeFunc:        s.AddNode,
		UpdateNodeFunc:     s.UpdateNode,
		DeleteNodeFunc:     s.DeleteNode,
		GetStatusFunc:      s.GetStatus,
	}, nil
}

type NfsStorage struct {
	client *clientset.Clientset
}

func (st *NfsStorage) Init() error {
	// Nothing to initialize, no external managing containers to check, very simple.
	return nil
}

func (st *NfsStorage) MakeDeployment(c *spec.StorageCluster,
	s *spec.StorageNode,
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

func (st *NfsStorage) AddNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug().Log("msg", "add node", "storagenode", s.Name)
	return nil, nil
}

func (st *NfsStorage) UpdateNode(c *spec.StorageCluster, s *spec.StorageNode) (*spec.StorageNode, error) {
	logger.Debug().Log("msg", "update node", "storagenode", s.Name)
	return nil, nil
}

func (st *NfsStorage) DeleteNode(c *spec.StorageCluster, s *spec.StorageNode) error {
	logger.Debug().Log("msg", "delete node", "storagenode", s.Name)
	return nil
}

func (st *NfsStorage) GetStatus(c *spec.StorageCluster) (*spec.StorageStatus, error) {
	logger.Debug().Log("msg", "status", "cluster", c.Name)
	status := &spec.StorageStatus{}
	return status, nil
}

func (st *NfsStorage) Type() spec.StorageTypeIdentifier {
	return spec.StorageTypeIdentifierNFS
}
