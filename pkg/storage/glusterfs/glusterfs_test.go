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
	"reflect"
	"testing"

	"github.com/coreos-inc/quartermaster/pkg/operator"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	"github.com/coreos-inc/quartermaster/pkg/tests"
	"github.com/coreos-inc/quartermaster/pkg/utils"

	qmstorage "github.com/coreos-inc/quartermaster/pkg/storage"

	"github.com/heketi/heketi/pkg/heketitest"

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

func getGlusterStorageFromStorageOperator(o qmstorage.StorageType) *GlusterStorage {
	sfns := o.(*qmstorage.StorageHandlerFuncs)
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

func TestGlusterFSMakeDeployment(t *testing.T) {
	n := &spec.StorageNode{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "StorageNode",
			APIVersion: operator.TPRVersion,
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels: map[string]string{
				"sample": "label",
			},
		},
		Spec: spec.StorageNodeSpec{
			Type:     "glusterfs",
			Image:    "myfakeimage",
			NodeName: "mynode",
			NodeSelector: map[string]string{
				"my": "node",
			},
		},
	}

	op, err := New(&clientset.Clientset{}, &restclient.RESTClient{})
	tests.Assert(t, err == nil)
	tests.Assert(t, op != nil)

	// Get a deployment.No old deployment
	// image is provided
	deploy, err := op.MakeDeployment(&spec.StorageCluster{}, n, nil)
	tests.Assert(t, err == nil)
	tests.Assert(t, deploy != nil)

	// Test labels
	n.Labels["quartermaster"] = n.Name
	tests.Assert(t, reflect.DeepEqual(deploy.Labels, n.Labels))

	// Test
	tests.Assert(t, deploy.Name == n.Name)
	tests.Assert(t, deploy.Namespace == n.Namespace)
	tests.Assert(t, deploy.Spec.Replicas == 1)
	tests.Assert(t, deploy.Spec.Template.Labels["quartermaster"] == n.Name)
	tests.Assert(t, deploy.Spec.Template.Name == n.Name)
	tests.Assert(t, reflect.DeepEqual(deploy.Spec.Template.Spec.NodeSelector, n.Spec.NodeSelector))
	tests.Assert(t, deploy.Spec.Template.Spec.NodeName == n.Spec.NodeName)

	tests.Assert(t, len(deploy.Spec.Template.Spec.Containers) == 1)
	tests.Assert(t, deploy.Spec.Template.Spec.Containers[0].Image == n.Spec.Image)

	// test volume mounts
}
