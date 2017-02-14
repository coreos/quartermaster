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

package client

import (
	"github.com/coreos-inc/quartermaster/pkg/spec"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type StorageClusters struct {
	t *transport
}

func NewStorageClusters(c restclient.Interface, namespace string) *StorageClusters {
	return &StorageClusters{
		t: newTransport(c, namespace, "storageclusters"),
	}
}

func (c *StorageClusters) Create(storageCluster *spec.StorageCluster) (result *spec.StorageCluster, err error) {
	obj, err := c.t.Create(storageCluster, &spec.StorageCluster{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageCluster), err
}

func (c *StorageClusters) Update(storageCluster *spec.StorageCluster) (result *spec.StorageCluster, err error) {
	obj, err := c.t.Update(storageCluster, storageCluster.GetName(), &spec.StorageCluster{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageCluster), err
}

func (c *StorageClusters) Delete(name string, options *api.DeleteOptions) error {
	return c.t.Delete(name, options)
}

func (c *StorageClusters) Get(name string) (result *spec.StorageCluster, err error) {
	obj, err := c.t.Get(name, &spec.StorageCluster{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageCluster), err
}

func (c *StorageClusters) List(opts api.ListOptions) (result *spec.StorageClusterList, err error) {
	obj, err := c.t.List(&spec.StorageClusterList{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageClusterList), err
}
