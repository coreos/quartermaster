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
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"github.com/coreos/quartermaster/pkg/operator"
	"github.com/coreos/quartermaster/pkg/spec"
	qmstorage "github.com/coreos/quartermaster/pkg/storage"
	"github.com/heketi/tests"
	"github.com/heketi/utils"

	heketiclient "github.com/heketi/heketi/client/api/go-client"
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

func objectToJSONBody(object interface{}) (io.ReadCloser, error) {
	j, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(j)), nil
}

func getGlusterStorageFromStorageOperator(o qmstorage.StorageType) *GlusterStorage {
	sfns := o.(*qmstorage.StorageHandlerFuncs)
	return sfns.StorageHandler.(*GlusterStorage)
}

// Create fake service
func getHeketiServiceObject(t *testing.T, namespace, fakeURL string) *api.Service {
	u, err := url.Parse(fakeURL)
	tests.Assert(t, err == nil)

	hostname, portstring, err := net.SplitHostPort(u.Host)
	tests.Assert(t, err == nil)
	porti, err := strconv.Atoi(portstring)
	tests.Assert(t, err == nil)
	port := int32(porti)

	return &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name:      "heketi",
			Namespace: namespace,
		},
		Spec: api.ServiceSpec{
			ClusterIP: hostname,
			Ports: []api.ServicePort{
				{
					Port: port,
				},
			},
		},
	}
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

	// Don't wait for deployemnt
	defer tests.Patch(&waitForDeploymentFn,
		func(client clientset.Interface, namespace, name string, available int32) error {
			return nil
		}).Restore()

	client := fakeclientset.NewSimpleClientset(
		getHeketiServiceObject(t, "test", "http://thisAddressDoesNotExist:1234"))
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
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

	// Don't wait for deployemnt
	defer tests.Patch(&waitForDeploymentFn,
		func(client clientset.Interface, namespace, name string, available int32) error {
			return nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset(
		getHeketiServiceObject(t, "test", heketiServer.URL()))
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	retc, err := op.AddCluster(c)
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

	// Don't wait for deployemnt
	defer tests.Patch(&waitForDeploymentFn,
		func(client clientset.Interface, namespace, name string, available int32) error {
			return nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset(
		getHeketiServiceObject(t, "test", heketiServer.URL()))
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
	deploy, err := op.MakeDeployment(n, nil)
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

func TestGlusterFSAddNewNodeWithHeketi(t *testing.T) {
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
			StorageNetwork: &spec.StorageNodeNetwork{
				IPs: []string{"1.1.1.1"},
			},
			NodeSelector: map[string]string{
				"my": "node",
			},
			ClusterRef: &api.ObjectReference{
				Name: c.Name,
			},
		},
	}

	// Setup fake Heketi service
	heketiServer := heketitest.NewHeketiMockTestServerDefault()
	defer heketiServer.Close()

	// Don't wait for deployemnt
	defer tests.Patch(&waitForDeploymentFn,
		func(client clientset.Interface, namespace, name string, available int32) error {
			return nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset(
		getHeketiServiceObject(t, "test", heketiServer.URL()))
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
		Client: fakerestclient.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" && req.URL.Path == "/namespaces/test/storageclusters/test" {
					resp := &http.Response{
						StatusCode: http.StatusOK,
					}

					rc, err := objectToJSONBody(c)
					tests.Assert(t, err == nil)

					header := http.Header{}
					header.Set("Content-Type", "application/json; charset=UTF-8")

					resp.Body = rc
					resp.Header = header

					return resp, nil
				} else {
					tests.Assert(t, false, "Unexpected request", req)
				}

				return nil, nil
			}),
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	// Add the node without initializing the cluster
	retn, err := op.AddNode(n)
	tests.Assert(t, err != nil)
	tests.Assert(t, retn == nil)

	// Add the cluster
	tests.Assert(t, c.Spec.GlusterFS == nil)
	retc, err := op.AddCluster(c)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(retc.Spec.GlusterFS.Cluster) != 0)

	// Add node .. missing glusterfs information
	retn, err = op.AddNode(n)
	tests.Assert(t, err != nil)
	tests.Assert(t, retn == nil)

	// Add node with glusterfs information
	n.Spec.GlusterFS = &spec.GlusterStorageNode{
		Zone: 1,
	}

	retn, err = op.AddNode(n)
	tests.Assert(t, err == nil)
	tests.Assert(t, retn != nil)

}

func TestGlusterFSAddNewNodeWithDevices(t *testing.T) {
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
			StorageNetwork: &spec.StorageNodeNetwork{
				IPs: []string{"1.1.1.1"},
			},
			ClusterRef: &api.ObjectReference{
				Name: c.Name,
			},
			Devices: []string{
				"/dev/fake1",
				"/dev/fake2",
				"/dev/fake3",
			},
			GlusterFS: &spec.GlusterStorageNode{
				Zone: 20,
			},
		},
	}

	// Setup fake Heketi service
	heketiServer := heketitest.NewHeketiMockTestServerDefault()
	defer heketiServer.Close()

	// Don't wait for deployemnt
	defer tests.Patch(&waitForDeploymentFn,
		func(client clientset.Interface, namespace, name string, available int32) error {
			return nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset(
		getHeketiServiceObject(t, "test", heketiServer.URL()))
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
		Client: fakerestclient.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" && req.URL.Path == "/namespaces/test/storageclusters/test" {
					resp := &http.Response{
						StatusCode: http.StatusOK,
					}

					rc, err := objectToJSONBody(c)
					tests.Assert(t, err == nil)

					header := http.Header{}
					header.Set("Content-Type", "application/json; charset=UTF-8")

					resp.Body = rc
					resp.Header = header

					return resp, nil
				} else {
					tests.Assert(t, false, "Unexpected request", req)
				}

				return nil, nil
			}),
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	// Add the cluster
	tests.Assert(t, c.Spec.GlusterFS == nil)
	retc, err := op.AddCluster(c)
	tests.Assert(t, err == nil)
	tests.Assert(t, retc != nil)

	// Add node
	retn, err := op.AddNode(n)
	tests.Assert(t, err == nil)
	tests.Assert(t, retn != nil)

	// Get Node information to check devices were registered
	h := heketiclient.NewClientNoAuth(heketiServer.URL())
	tests.Assert(t, h != nil)
	nodeInfo, err := h.NodeInfo(retn.Spec.GlusterFS.Node)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(retn.Spec.Devices) == len(nodeInfo.DevicesInfo))

	for _, device := range retn.Spec.Devices {
		found := false
		for _, check := range nodeInfo.DevicesInfo {
			if device == check.Name {
				found = true
				break
			}
		}
		tests.Assert(t, found, device, nodeInfo.DevicesInfo)
	}
}

func TestGlusterFSAddNewNodeAddOneDevice(t *testing.T) {
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
			StorageNetwork: &spec.StorageNodeNetwork{
				IPs: []string{"1.1.1.1"},
			},
			ClusterRef: &api.ObjectReference{
				Name: c.Name,
			},
			Devices: []string{
				"/dev/fake1",
				"/dev/fake2",
				"/dev/fake3",
			},
			GlusterFS: &spec.GlusterStorageNode{
				Zone: 20,
			},
		},
	}

	// Setup fake Heketi service
	heketiServer := heketitest.NewHeketiMockTestServerDefault()
	defer heketiServer.Close()

	// Don't wait for deployemnt
	defer tests.Patch(&waitForDeploymentFn,
		func(client clientset.Interface, namespace, name string, available int32) error {
			return nil
		}).Restore()

	// Setup fake Kube clients
	client := fakeclientset.NewSimpleClientset(
		getHeketiServiceObject(t, "test", heketiServer.URL()))
	rclient := &fakerestclient.RESTClient{
		NegotiatedSerializer: serializer.DirectCodecFactory{CodecFactory: api.Codecs},
		Client: fakerestclient.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" && req.URL.Path == "/namespaces/test/storageclusters/test" {
					resp := &http.Response{
						StatusCode: http.StatusOK,
					}

					rc, err := objectToJSONBody(c)
					tests.Assert(t, err == nil)

					header := http.Header{}
					header.Set("Content-Type", "application/json; charset=UTF-8")

					resp.Body = rc
					resp.Header = header

					return resp, nil
				} else {
					tests.Assert(t, false, "Unexpected request", req)
				}

				return nil, nil
			}),
	}
	op, err := New(client, rclient)
	tests.Assert(t, err == nil)

	// Add the cluster
	tests.Assert(t, c.Spec.GlusterFS == nil)
	retc, err := op.AddCluster(c)
	tests.Assert(t, err == nil)
	tests.Assert(t, retc != nil)

	// Add node
	retn, err := op.AddNode(n)
	tests.Assert(t, err == nil)
	tests.Assert(t, retn != nil)

	// Get Node information to check devices were registered
	h := heketiclient.NewClientNoAuth(heketiServer.URL())
	tests.Assert(t, h != nil)
	nodeInfo, err := h.NodeInfo(retn.Spec.GlusterFS.Node)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(retn.Spec.Devices) == len(nodeInfo.DevicesInfo))

	for _, device := range retn.Spec.Devices {
		found := false
		for _, check := range nodeInfo.DevicesInfo {
			if device == check.Name {
				found = true
				break
			}
		}
		tests.Assert(t, found, device, nodeInfo.DevicesInfo)
	}

	// Now add one device
	retn.Spec.Devices = append(retn.Spec.Devices, "/dev/added")

	// Add new device to node
	retn, err = op.AddNode(retn)
	tests.Assert(t, err == nil)
	tests.Assert(t, retn != nil)

	// Get Node information to check the new device was added
	nodeInfo, err = h.NodeInfo(retn.Spec.GlusterFS.Node)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(retn.Spec.Devices) == len(nodeInfo.DevicesInfo))

	for _, device := range retn.Spec.Devices {
		found := false
		for _, check := range nodeInfo.DevicesInfo {
			if device == check.Name {
				found = true
				break
			}
		}
		tests.Assert(t, found, device, nodeInfo.DevicesInfo)
	}
}
