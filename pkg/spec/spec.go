// Copyright 2016 The quartermaster Authors
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

package spec

import (
	"k8s.io/client-go/pkg/api/v1"
	metav1 "k8s.io/client-go/pkg/apis/meta/v1"
)

// StorageNode defines a single instance of available storage on a
// node and the appropriate options to apply to it to make it available
// to the cluster.
type StorageNode struct {
	metav1.TypeMeta `json:",inline"`
	v1.ObjectMeta   `json:"metadata,omitempty"`
	Spec            StorageNodeSpec `json:"spec"`
}

// StorageNodeList is a list of StorageNode objects in Kubernetes.
type StorageNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []*StorageNode `json:"items"`
}

// StorageNodeSpec holds specification parameters for a StorageNode.
type StorageNodeSpec struct {
	Image          string              `json:"image,omitempty"` // Non-default container image to use for this storage node.
	Type           string              `json:"type"`            // Storage Implementation to use atop this storage.
	NodeSelector   map[string]string   `json:"nodeSelector"`
	StorageNetwork *StorageNodeNetwork `json:"storageNetwork"`
	Devices        []string            `json:"devices"`     // Raw block devices available on the StorageNode to be used for storage.
	Directories    []string            `json:"directories"` // Directory-based storage available on the StorageNode to be used for storage.
	GlusterFS      *GlusterStorageNode `json:"glusterfs"`
	Torus          *TorusStorageNode   `json:"torus"`
	NFS            *NFSStorageNode     `json:"nfs"`
}

// StorageNodeNetwork specifies which network interfaces the StorageNode should
// use for data transport, which may be separate from it's Kubernetes-accessible
// IP.
type StorageNodeNetwork struct {
	IPs []string `json:"ips"`
}

// GlusterStorageNode defines the specifics of how this Gluster instance should be instantiated.
type GlusterStorageNode struct {
	Cluster string `json:"cluster"` // Cluster ID this node should belong to.
	Zone    string `json:"zone"`    // Zone ID this node belongs to. If missing, Zone 1 will be assumed.
}

// TorusStorageNode defines the specifics of how this Gluster instance should be instantiated.
type TorusStorageNode struct {
	MonitorPort string `json:"monitorPort"`
}

// NFSStorageNode defines the specifics of how this Gluster instance should be instantiated.
type NFSStorageNode struct {
	// TODO(barakmich): More NFS options as per `man nfs` or `man exports`.
	ReadOnly     bool `json:"readOnly"`     // This node exports NFS volumes ReadOnly.
	SubtreeCheck bool `json:"subtreeCheck"` // Enable mount subtree checking on the host.
	NoRootSquash bool `json:"noRootSquash"` // Disable root squashing, mapping UID 0 in the client to UID 0 on the host.
}

// StorageStatus reports on the status of a storage deployment backend.
type StorageStatus struct {
	metav1.TypeMeta `json:",inline"`
	v1.ObjectMeta   `json:"metadata,omitempty"`
	Details         map[string]string `json:"details"`
}
