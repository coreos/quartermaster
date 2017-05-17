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

package operator

import (
	"net/url"
	"time"

	"github.com/coreos/quartermaster/pkg/spec"
	qmstorage "github.com/coreos/quartermaster/pkg/storage"
	"github.com/heketi/utils"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/labels"
)

var (
	logger = utils.NewLogger("operator", utils.LEVEL_DEBUG)
)

type Operator struct {
	kclient        *clientset.Clientset
	rclient        *restclient.RESTClient
	storageSystems map[spec.StorageTypeIdentifier]qmstorage.StorageType
	clusterOp      StorageOperator
	nodeOp         StorageOperator
	queue          *queue
	host           string
}

// Config defines configuration parameters for the Operator.
type Config struct {
	Host        string
	TLSInsecure bool
	TLSConfig   restclient.TLSClientConfig
}

// New creates a new controller.
func New(c Config, storageFuns ...qmstorage.StorageTypeNewFunc) (*Operator, error) {
	cfg, err := newClusterConfig(c.Host, c.TLSInsecure, &c.TLSConfig)
	if err != nil {
		return nil, err
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	rclient, err := NewQuartermasterRESTClient(*cfg)
	if err != nil {
		return nil, err
	}

	// Initialize storage plugins
	storageSystems := make(map[spec.StorageTypeIdentifier]qmstorage.StorageType)
	for _, newStorage := range storageFuns {

		// New
		st, err := newStorage(client, rclient)
		if err != nil {
			return nil, err
		}

		// Save object
		storageSystems[st.Type()] = st

		logger.Info("storage driver %v loaded", st.Type())
	}

	return &Operator{
		kclient:        client,
		rclient:        rclient,
		queue:          newQueue(200),
		host:           cfg.Host,
		storageSystems: storageSystems,
	}, nil
}

func (c *Operator) GetStorage(name spec.StorageTypeIdentifier) (qmstorage.StorageType, error) {
	if storage, ok := c.storageSystems[name]; ok {
		return storage, nil
	} else {
		return nil, logger.LogError("invalid storage type: %v", name)
	}
}

// Run the controller.
func (c *Operator) Run(stopc <-chan struct{}) error {
	defer c.queue.close()

	go c.worker()

	// Test communication with server
	v, err := c.kclient.Discovery().ServerVersion()
	if err != nil {
		return logger.LogError("communicating with server failed: %s", err)
	}
	logger.Info("connection to Kubernetes established. Cluster version %v", v)

	// Create ThirdPartyResources
	if err := c.createTPRs(); err != nil {
		return logger.LogError("Unable to create TPR: %v", err)
	}

	// Setup StorageCluster Operator
	c.clusterOp = NewStorageClusterOperator(c)
	c.clusterOp.Setup(stopc)

	// Setup StorageNode Operator
	c.nodeOp = NewStorageNodeOperator(c)
	c.nodeOp.Setup(stopc)

	// Wait until event handlers are ready
	logger.Info("Waiting for synchronization with Kubernetes TPR")
	for !c.nodeOp.HasSynced() ||
		!c.clusterOp.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}
	logger.Info("Synchronization complete")

	<-stopc
	return nil
}

type queue struct {
	ch chan *spec.StorageNode
}

func newQueue(size int) *queue {
	return &queue{ch: make(chan *spec.StorageNode, size)}
}

func (q *queue) add(p *spec.StorageNode) { q.ch <- p }
func (q *queue) close()                  { close(q.ch) }

func (q *queue) pop() (*spec.StorageNode, bool) {
	p, ok := <-q.ch
	return p, ok
}

var keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Operator) worker() {
	for {
		_, ok := c.queue.pop()
		if !ok {
			return
		}
	}
}

func (c *Operator) GetRESTClient() *restclient.RESTClient {
	return c.rclient
}

func ListOptions(name string) api.ListOptions {
	s := labels.SelectorFromSet(map[string]string{
		"quartermaster": name,
	})
	return api.ListOptions{
		LabelSelector: s,
	}
}

func newClusterConfig(host string, tlsInsecure bool, tlsConfig *restclient.TLSClientConfig) (*restclient.Config, error) {
	var cfg *restclient.Config
	var err error

	if len(host) == 0 {
		if cfg, err = restclient.InClusterConfig(); err != nil {
			return nil, err
		}
	} else {
		cfg = &restclient.Config{
			Host: host,
		}
		hostURL, err := url.Parse(host)
		if err != nil {
			return nil, logger.LogError("error parsing host url %s : %v", host, err)
		}
		if hostURL.Scheme == "https" {
			cfg.TLSClientConfig = *tlsConfig
			cfg.Insecure = tlsInsecure
		}
	}
	cfg.QPS = 100
	cfg.Burst = 100

	return cfg, nil
}
