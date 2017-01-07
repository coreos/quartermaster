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
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/coreos-inc/quartermaster/pkg/spec"

	"github.com/go-kit/kit/log"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

// Operator manages lify cycle of Prometheus deployments and
// monitoring configurations.
type Operator struct {
	kclient *clientset.Clientset
	rclient *restclient.RESTClient
	logger  log.Logger

	storageSystems map[spec.StorageTypeIdentifier]StorageType

	nodeInf   cache.SharedIndexInformer
	dsetInf   cache.SharedIndexInformer
	clusterOp StorageOperator

	queue *queue

	host string
}

// Config defines configuration parameters for the Operator.
type Config struct {
	Host        string
	TLSInsecure bool
	TLSConfig   restclient.TLSClientConfig
}

// New creates a new controller.
func New(c Config, storageFuns ...StorageTypeNewFunc) (*Operator, error) {
	cfg, err := newClusterConfig(c.Host, c.TLSInsecure, &c.TLSConfig)
	if err != nil {
		return nil, err
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	logger := log.NewContext(log.NewLogfmtLogger(os.Stdout)).
		With("ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	rclient, err := newQuartermasterRESTClient(*cfg)
	if err != nil {
		return nil, err
	}

	// Initialize storage plugins
	storageSystems := make(map[spec.StorageTypeIdentifier]StorageType)
	for _, newStorage := range storageFuns {

		// New
		st, err := newStorage(client)
		if err != nil {
			return nil, err
		}

		// Save object
		storageSystems[st.Type()] = st

		logger.Log("msg", "storage loaded", "type", st.Type())
	}

	return &Operator{
		kclient:        client,
		rclient:        rclient,
		logger:         logger,
		queue:          newQueue(200),
		host:           cfg.Host,
		storageSystems: storageSystems,
	}, nil
}

func (c *Operator) GetStorage(name spec.StorageTypeIdentifier) (StorageType, error) {
	if storage, ok := c.storageSystems[name]; ok {
		return storage, nil
	} else {
		return nil, c.logger.Log("invalid storage type", name)
	}
}

// Run the controller.
func (c *Operator) Run(stopc <-chan struct{}) error {
	defer c.queue.close()

	// Start notification worker
	go c.worker()

	// :TODO: (lpabon) This needs to be orginized better
	//         with the other worker.
	// go c.clusterWorker()
	c.clusterOp = NewStorageClusterOperator(c)
	c.clusterOp.Setup(stopc)

	// Test communication with server
	v, err := c.kclient.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("communicating with server failed: %s", err)
	}
	c.logger.Log("msg", "connection established", "cluster-version", v)

	// Create ThirdPartyResources
	if err := c.createTPRs(); err != nil {
		return c.logger.Log("msg", "unable to create tpr", "err", err)
	}

	// Create notification objects
	c.nodeInf = cache.NewSharedIndexInformer(
		NewStorageNodeListWatch(c.rclient),
		&spec.StorageNode{}, resyncPeriod, cache.Indexers{})
	c.dsetInf = cache.NewSharedIndexInformer(
		cache.NewListWatchFromClient(c.kclient.Extensions().RESTClient(), "deployments", api.NamespaceAll, nil),
		&extensions.Deployment{}, resyncPeriod, cache.Indexers{})

	// Register Handlers
	c.logger.Log("msg", "Register event handlers")
	c.nodeInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(p interface{}) {
			c.logger.Log("msg", "enqueueStorageNode", "trigger", "storagenode add")
			c.enqueueStorageNode(p)
		},
		DeleteFunc: func(p interface{}) {
			c.logger.Log("msg", "enqueueStorageNode", "trigger", "storagenode del")
			c.enqueueStorageNode(p)
		},
		UpdateFunc: func(_, p interface{}) {
			c.logger.Log("msg", "enqueueStorageNode", "trigger", "storagenode update")
			c.enqueueStorageNode(p)
		},
	})
	c.dsetInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(d interface{}) {
			c.logger.Log("msg", "addDeployment", "trigger", "deployment add")
			c.addDeployment(d)
		},
		DeleteFunc: func(d interface{}) {
			c.logger.Log("msg", "deleteDeployment", "trigger", "deployment delete")
			c.deleteDeployment(d)
		},
		UpdateFunc: func(old, cur interface{}) {
			c.logger.Log("msg", "updateDeployment", "trigger", "deployment update")
			c.updateDeployment(old, cur)
		},
	})

	go c.nodeInf.Run(stopc)
	go c.dsetInf.Run(stopc)

	c.logger.Log("msg", "Waiting for sync")
	for !c.nodeInf.HasSynced() ||
		!c.dsetInf.HasSynced() ||
		!c.clusterOp.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}
	c.logger.Log("msg", "Sync done")

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

func (c *Operator) enqueueStorageNode(p interface{}) {
	c.queue.add(p.(*spec.StorageNode))
}

func (c *Operator) enqueueStorageNodeIf(f func(p *spec.StorageNode) bool) {
	cache.ListAll(c.nodeInf.GetStore(), labels.Everything(), func(o interface{}) {
		if f(o.(*spec.StorageNode)) {
			c.enqueueStorageNode(o.(*spec.StorageNode))
		}
	})
}

func (c *Operator) enqueueAll() {
	cache.ListAll(c.nodeInf.GetStore(), labels.Everything(), func(o interface{}) {
		c.enqueueStorageNode(o.(*spec.StorageNode))
	})
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Operator) worker() {
	for {
		p, ok := c.queue.pop()
		if !ok {
			return
		}
		if err := c.reconcile(p); err != nil {
			utilruntime.HandleError(fmt.Errorf("reconciliation failed: %s", err))
		}
	}
}

func (c *Operator) storageNodeForDeployment(d *extensions.Deployment) *spec.StorageNode {
	key, err := keyFunc(d)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("creating key: %s", err))
		return nil
	}
	// Namespace/Name are one-to-one so the key will find the respective Prometheus resource.
	s, exists, err := c.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("get Prometheus resource: %s", err))
		return nil
	}
	if !exists {
		return nil
	}
	return s.(*spec.StorageNode)
}

func (c *Operator) deleteDeployment(o interface{}) {
	d := o.(*extensions.Deployment)
	if s := c.storageNodeForDeployment(d); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *Operator) addDeployment(o interface{}) {
	d := o.(*extensions.Deployment)
	if s := c.storageNodeForDeployment(d); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *Operator) updateDeployment(oldo, curo interface{}) {
	old := oldo.(*extensions.Deployment)
	cur := curo.(*extensions.Deployment)

	c.logger.Log("msg", "update handler", "old", old.ResourceVersion, "cur", cur.ResourceVersion)

	// Periodic resync may resend the deployment without changes in-between.
	// Also breaks loops created by updating the resource ourselves.
	if old.ResourceVersion == cur.ResourceVersion {
		return
	}

	if s := c.storageNodeForDeployment(cur); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *Operator) reconcile(s *spec.StorageNode) error {
	key, err := keyFunc(s)
	if err != nil {
		return err
	}
	c.logger.Log("msg", "reconcile storagenode", "key", key)

	_, exists, err := c.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		return err
	}

	storage, err := c.GetStorage(s.Spec.Type)
	if err != nil {
		return err
	}

	if !exists {
		// TODO(fabxc): we want to do server side deletion due to the variety of
		// resources we create.
		// Doing so just based on the deletion event is not reliable, so
		// we have to garbage collect the controller-created resources in some other way.
		//
		// Let's rely on the index key matching that of the created configmap and replica
		// set for now. This does not work if we delete Prometheus resources as the
		// controller is not running – that could be solved via garbage collection later.
		err := storage.DeleteNode(&spec.StorageCluster{}, s)
		if err != nil {
			return c.logger.Log("err", err)
		}

		reaper, err := kubectl.ReaperFor(extensions.Kind("Deployment"), c.kclient)
		if err != nil {
			return c.logger.Log("err", err)
		}

		err = reaper.Stop(s.Namespace, s.Name, time.Minute, api.NewDeleteOptions(0))
		if err != nil {
			return c.logger.Log("err", err)
		}

	}

	dsetClient := c.kclient.Extensions().Deployments(s.Namespace)
	// Ensure we have a replica set running Prometheus deployed.
	// XXX: Selecting by ObjectMeta.Name gives an error. So use the label for now.
	deployment := &extensions.Deployment{}
	deployment.Namespace = s.Namespace
	deployment.Name = s.Name
	obj, exists, err := c.dsetInf.GetStore().Get(deployment)
	if err != nil {
		return err
	}

	if !exists {

		// Get a deployment from plugin
		ds, err := storage.MakeDeployment(&spec.StorageCluster{}, s, nil)
		if err != nil {
			return err
		}

		// Plugin may not need a deployment
		if ds != nil {
			if _, err := dsetClient.Create(ds); err != nil {
				return fmt.Errorf("create deployment: %s", err)
			}
		}
		// :TODO: Wait until it is ready

		// Add node
		_, err = storage.AddNode(&spec.StorageCluster{}, s)
		if err != nil {
			// :TODO: Clean up and delete Deployment

			return err
		}

	} else {
		// Update
		ds, err := storage.MakeDeployment(&spec.StorageCluster{},
			s,
			obj.(*extensions.Deployment))
		if err != nil {
			return err
		}

		// Plugin may not need a deployment
		if ds != nil {
			// TODO(barakmich): This may be broken for DaemonSets.
			// Will be fixed when DaemonSets do rolling updates.
			if _, err := dsetClient.Update(ds); err != nil {
				return err
			}
		}

		// Update Node
		_, err = storage.UpdateNode(&spec.StorageCluster{}, s)
		if err != nil {
			return err
		}
	}

	return c.updateStatus()
}

func (c *Operator) updateStatus() error {
	// TODO(barakmich): Call into c.storage.GetStatus()
	return nil
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
			return nil, fmt.Errorf("error parsing host url %s : %v", host, err)
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
