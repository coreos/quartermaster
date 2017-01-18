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
	"net/http"
	"testing"

	"github.com/coreos-inc/quartermaster/pkg/operator"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	"github.com/heketi/heketi/pkg/heketitest"
	"github.com/heketi/tests"
	"github.com/heketi/utils"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	fakeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/restclient"
	fakerestclient "k8s.io/kubernetes/pkg/client/restclient/fake"
	"k8s.io/kubernetes/pkg/runtime/serializer"
)

func init() {
	logger.SetLevel(utils.LEVEL_NOLOG)
}

func getGlusterStorageFromStorageOperator(o operator.StorageType) *GlusterStorage {
	sfns := o.(*operator.StorageHandlerFuncs)
	return sfns.StorageHandler.(*GlusterStorage)
}

func TestNewGlusterFSStorage(t *testing.T) {
	c := &clientset.Clientset{}
	r := &restclient.RESTClient{}

	op, err := New(c, r)
	tests.Assert(t, err == nil)

	gs := getGlusterStorageFromStorageOperator(op)
	tests.Assert(t, gs.client == c)
	tests.Assert(t, gs.qm == r)
}

func TestGlusterFSInit(t *testing.T) {
	c := &fakeclientset.Clientset{}
	r := &restclient.RESTClient{}

	op, err := New(c, r)
	tests.Assert(t, err == nil)

	gs := getGlusterStorageFromStorageOperator(op)
	err = gs.Init()
	tests.Assert(t, err == nil)
}

func TestGlusterFSAddClusterNoHeketi(t *testing.T) {
	c := &spec.StorageCluster{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "StorageCluster",
			APIVersion: operator.TPRVersion,
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: spec.StorageClusterSpec{
			Type: "glusterfs",
		},
	}
	client := fakeclientset.NewSimpleClientset()
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
		Client: fakerestclient.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				return nil, nil
			}),
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	retc, err := op.AddCluster(c)
	tests.Assert(t, err != nil, err)
	tests.Assert(t, retc == nil)
}

func TestGlusterFSAddNewClusterWithHeketi(t *testing.T) {
	c := &spec.StorageCluster{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "StorageCluster",
			APIVersion: operator.TPRVersion,
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: spec.StorageClusterSpec{
			Type: "glusterfs",
		},
	}

	// Setup fake Heketi service
	heketiServer := heketitest.NewHeketiMockTestServerDefault()
	defer heketiServer.Close()

	// Set server
	called := 0
	defer tests.Patch(&heketiAddressFn,
		func(namespace string) (string, error) {
			called++
			return heketiServer.URL(), nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset()
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
		Client: fakerestclient.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				return nil, nil
			}),
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	retc, err := op.AddCluster(c)
	tests.Assert(t, called == 1)
	tests.Assert(t, err == nil)
	tests.Assert(t, retc != nil)
	tests.Assert(t, len(retc.Spec.GlusterFS.Cluster) != 0)
}

func TestGlusterFSExistingClusterWithHeketi(t *testing.T) {
	c := &spec.StorageCluster{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "StorageCluster",
			APIVersion: operator.TPRVersion,
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: spec.StorageClusterSpec{
			Type: "glusterfs",
			GlusterFS: &spec.GlusterStorageCluster{
				Cluster: "ABC",
			},
		},
	}

	// Setup fake Heketi service
	heketiServer := heketitest.NewHeketiMockTestServerDefault()
	defer heketiServer.Close()

	// Set server
	called := 0
	defer tests.Patch(&heketiAddressFn,
		func(namespace string) (string, error) {
			called++
			return heketiServer.URL(), nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset()
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
		Client: fakerestclient.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				return nil, nil
			}),
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	retc, err := op.AddCluster(c)
	tests.Assert(t, called == 0)
	tests.Assert(t, err == nil)
	tests.Assert(t, retc == nil)
	tests.Assert(t, c.Spec.GlusterFS.Cluster == "ABC")
}
