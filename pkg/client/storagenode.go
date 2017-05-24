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
	"github.com/coreos/quartermaster/pkg/spec"

	"k8s.io/client-go/pkg/api"
	restclient "k8s.io/client-go/rest"
)

type StorageNodes struct {
	t *transport
}

func NewStorageNodes(c restclient.Interface, namespace string) *StorageNodes {
	return &StorageNodes{
		t: newTransport(c, namespace, "storagenodes"),
	}
}

func (c *StorageNodes) Create(storageNode *spec.StorageNode) (result *spec.StorageNode, err error) {
	obj, err := c.t.Create(storageNode, &spec.StorageNode{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageNode), err
}

func (c *StorageNodes) Update(storageNode *spec.StorageNode) (result *spec.StorageNode, err error) {
	obj, err := c.t.Update(storageNode, storageNode.Name, &spec.StorageNode{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageNode), err
}

func (c *StorageNodes) Delete(name string, options *api.DeleteOptions) error {
	return c.t.Delete(name, options)
}

func (c *StorageNodes) Get(name string) (result *spec.StorageNode, err error) {
	obj, err := c.t.Get(name, &spec.StorageNode{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageNode), err
}

func (c *StorageNodes) List(opts api.ListOptions) (result *spec.StorageNodeList, err error) {
	obj, err := c.t.List(&spec.StorageNodeList{})
	if err != nil {
		return nil, err
	}
	return obj.(*spec.StorageNodeList), err
}
