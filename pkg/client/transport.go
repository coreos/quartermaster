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
	"encoding/json"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type transport struct {
	resource string
	ns       string
	client   restclient.Interface
}

func newTransport(c restclient.Interface, namespace, resoure string) *transport {
	return &transport{
		client:   c,
		ns:       namespace,
		resource: resoure,
	}
}

func (t *transport) Create(in interface{}, resultObj interface{}) (result interface{}, err error) {
	result = resultObj

	req := t.client.Post().
		Namespace(t.ns).
		Resource(t.resource).
		Body(in)

	body, err := req.DoRaw()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, result)
	return
}

func (t *transport) Update(in interface{},
	name string,
	resultObj interface{}) (result interface{}, err error) {

	result = resultObj
	req := t.client.Put().
		Namespace(t.ns).
		Resource(t.resource).
		Name(name).
		Body(in)

	body, err := req.DoRaw()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, result)
	return
}

func (t *transport) Delete(name string, options *api.DeleteOptions) error {
	return t.client.Delete().
		Namespace(t.ns).
		Resource(t.resource).
		Name(name).
		Body(options).
		Do().
		Error()
}

func (t *transport) Get(name string, resultObj interface{}) (result interface{}, err error) {
	result = resultObj
	req := t.client.Get().
		Namespace(t.ns).
		Resource(t.resource).
		Name(name)

	body, err := req.DoRaw()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, result)
	return
}

func (t *transport) List(resultObj interface{}) (result interface{}, err error) {
	result = resultObj
	r := t.client.Get().
		Namespace(t.ns).
		Resource(t.resource)

	body, err := r.DoRaw()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, result)
	return
}
