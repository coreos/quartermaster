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

// This document contains all Quartermaster defined resources for Kubernetes
// Sections with @DRIVER highlight Kubernetes resources where drivers can
// add specific information

package spec

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageNode defines a single instance of available storage on a
// node and the appropriate options to apply to it to make it available
// to the cluster.
type StorageNode struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            StorageNodeSpec `json:"spec,omitempty"`

	// Status represents the current status of the storage node
	// +optional
	Status StorageNodeStatus `json:"status,omitempty"`
}

// Third Party Resource object which contains all the necessary information
// to deploy a storage cluster
type StorageCluster struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            StorageClusterSpec `json:"spec,omitempty"`

	// Status represents the current status of the storage node
	// +optional
	Status StorageClusterStatus `json:"status,omitempty"`
}

// StorageNodeList is a list of StorageNode objects in Kubernetes.
type StorageNodeList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`

	Items []StorageNode `json:"items"`
}

// StorageClusterList is a list of StorageCluster objects in Kubernetes
type StorageClusterList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`

	Items []StorageCluster `json:"items"`
}

type StorageTypeIdentifier string

// Add here the storage type identifier for your driver
// This is the storage type identifier administrators will use to identify
// which storage system to deploy.
// @DRIVER
const (
	StorageTypeIdentifierMock      StorageTypeIdentifier = "mock"
	StorageTypeIdentifierNFS       StorageTypeIdentifier = "nfs"
	StorageTypeIdentifierGlusterFS StorageTypeIdentifier = "glusterfs"
	StorageTypeIdentifierSwift     StorageTypeIdentifier = "swift"
)

// Specification for a StorageCluster
type StorageClusterSpec struct {
	// Software defined storage type
	Type StorageTypeIdentifier `json:"type,omitempty"`

	// Specific image to use on all nodes of the cluster.  If not avaiable,
	// it defaults to the image from QuarterMaster
	// +optional
	Image string `json:"image,omitempty"`

	// All nodes participating in this cluster
	StorageNodes []StorageNodeSpec `json:"storageNodes,omitempty"`

	// Add storage specific section here
	// @DRIVER

	GlusterFS *GlusterStorageCluster `json:"glusterfs,omitempty"`
}

// StorageNodeSpec holds specification parameters for a StorageNode.
type StorageNodeSpec struct {
	// Software defined storage type
	Type StorageTypeIdentifier `json:"type,omitempty"`

	// Specific image to use on the storage node requested.  If not avaiable,
	// it defaults to the StorageCluster image.
	// +optional
	Image string `json:"image,omitempty"`

	// Request the storage node be scheduled on a specific node
	// Must have set either Node or NodeSelector
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Request the storage node be scheduled on a node that matches the labels
	// Must have either Node or NodeSelector
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Storage network if any
	StorageNetwork *StorageNodeNetwork `json:"storageNetwork,omitempty"`

	// Raw block devices available on the StorageNode to be used for storage.
	// Devices or Directories must be set and their use are specific to
	// the implementation
	// Must have set either Devices or Directories
	// +optional
	Devices []string `json:"devices,omitempty"`

	// Directory-based storage available on the StorageNode to be used for storage.
	// Devices or Directories must be set and their use are specific to
	// the implementation
	// Must have set either Devices or Directories
	// +optional
	Directories []string `json:"directories,omitempty"`

	// Storage system settings
	// @DRIVER
	GlusterFS *GlusterStorageNode `json:"glusterfs,omitempty"`
	NFS       *NFSStorageNode     `json:"nfs,omitempty"`
}

// StorageNodeNetwork specifies which network interfaces the StorageNode should
// use for data transport, which may be separate from it's Kubernetes-accessible
// IP.
type StorageNodeNetwork struct {
	IPs []string `json:"ips"`
}

// GlusterStorageCluster defines the specific information about the cluster
type GlusterStorageCluster struct {
	HeketiImage string `json:"heketiImage"`
	Cluster     string `json:"cluster"`
}

// GlusterStorageNode defines the specifics of how this Gluster instance should be instantiated.
type GlusterStorageNode struct {
	Cluster string `json:"cluster"` // Cluster ID this node should belong to.
	Node    string `json:"node"`    // Node ID
	Zone    int    `json:"zone"`    // Zone ID this node belongs to. If missing, Zone 1 will be assumed.
}

// NFSStorageNode defines the specifics of how this Gluster instance should be instantiated.
type NFSStorageNode struct {
	ReadOnly bool   `json:"readOnly"` // This node exports NFS volumes ReadOnly.
	Size     string `json:"size"`     // Size in GB of share
}

type StatusCondition struct {
	Time    meta.Time `json:"time,omitempty"`
	Message string    `json:"message,omitempty"`
	Reason  string    `json:"reason,omitempty"`
}

type StatusInfo struct {
	Ready bool `json:"ready"`

	// The following follow the same definition as PodStatus
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type StorageClusterConditionType string

const (
	ClusterConditionReady   StorageClusterConditionType = "Ready"
	ClusterConditionOffline StorageClusterConditionType = "Offline"
)

type StorageClusterCondition struct {
	StatusCondition
	Type StorageClusterConditionType `json:"type,omitempty"`
}

type StorageNodeConditionType string

const (
	NodeConditionReady   StorageNodeConditionType = "Ready"
	NodeConditionOffline StorageNodeConditionType = "Offline"
)

type StorageNodeCondition struct {
	StatusCondition
	Type StorageNodeConditionType `json:"type,omitempty"`
}

type StorageClusterStatus struct {
	StatusInfo
	Conditions   []StorageClusterCondition `json:"conditions,omitempty"`
	NodeStatuses []StorageNodeStatus       `json:"nodeStatuses,omitempty"`

	// Add storage specific status
	// @DRIVER
}

type StorageNodeStatus struct {
	StatusInfo
	Added      bool                   `json:"added,omitempty"`
	Conditions []StorageNodeCondition `json:"conditions,omitempty"`
	PodName    string                 `json:"podName,omitempty"`
	NodeName   string                 `json:"nodeName,omitempty"`

	// Add storage specific status
	// @DRIVER
}
