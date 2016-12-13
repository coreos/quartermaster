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

package operator

import (
	"github.com/coreos-inc/quartermaster/pkg/spec"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type StorageType interface {
	Init() error
	MakeDaemonSet(s *spec.StorageNode, old *v1beta1.DaemonSet) (*v1beta1.DaemonSet, error)
	AddNode(s *spec.StorageNode) error
	GetStatus(s *spec.StorageNode) (*spec.StorageStatus, error)
}

type StorageTypeNewFunc func(*kubernetes.Clientset) (StorageType, error)
