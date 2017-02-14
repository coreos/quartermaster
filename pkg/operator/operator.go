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

	qmclient "github.com/coreos-inc/quartermaster/pkg/client"
	"github.com/coreos-inc/quartermaster/pkg/spec"
	qmstorage "github.com/coreos-inc/quartermaster/pkg/storage"
	"github.com/coreos-inc/quartermaster/pkg/utils"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

var (
	logger = utils.NewLogger("operator", utils.LEVEL_DEBUG)
)

type Operator struct {
	kclient        *clientset.Clientset
	rclient        *restclient.RESTClient
	storageSystems map[spec.StorageTypeIdentifier]qmstorage.StorageType
	nodeInf        cache.SharedIndexInformer
	dsetInf        cache.SharedIndexInformer
	clusterOp      StorageOperator
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

	// Start notification worker
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

	// Create notification objects
	c.nodeInf = cache.NewSharedIndexInformer(
		NewStorageNodeListWatch(c.rclient),
		&spec.StorageNode{}, resyncPeriod, cache.Indexers{})
	c.dsetInf = cache.NewSharedIndexInformer(
		cache.NewListWatchFromClient(c.kclient.Extensions().RESTClient(), "deployments", api.NamespaceAll, nil),
		&extensions.Deployment{}, resyncPeriod, cache.Indexers{})

	// Register Handlers
	logger.Debug("register event handlers")
	c.nodeInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(p interface{}) {
			logger.Debug("enqueueStorageNode trigger for storage add")
			c.enqueueStorageNode(p)
		},
		DeleteFunc: func(p interface{}) {
			logger.Debug("enqueueStorageNode trigger for storage del")
			c.enqueueStorageNode(p)
		},
		UpdateFunc: func(_, p interface{}) {
			logger.Debug("enqueueStorageNode trigger for storage update")
			c.enqueueStorageNode(p)
		},
	})
	c.dsetInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(d interface{}) {
			logger.Debug("addDeployment trigger for deployment add")
			c.addDeployment(d)
		},
		DeleteFunc: func(d interface{}) {
			logger.Debug("addDeployment trigger for deployment delete")
			c.deleteDeployment(d)
		},
		UpdateFunc: func(old, cur interface{}) {
			logger.Debug("addDeployment trigger for deployment update")
			c.updateDeployment(old, cur)
		},
	})

	// Setup StorageCluster Operator
	c.clusterOp = NewStorageClusterOperator(c)
	c.clusterOp.Setup(stopc)

	// Spawn event handlers
	go c.nodeInf.Run(stopc)
	go c.dsetInf.Run(stopc)

	// Wait until event handlers are ready
	logger.Info("Waiting for synchronization with Kubernetes TPR")
	for !c.nodeInf.HasSynced() ||
		!c.dsetInf.HasSynced() ||
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
			utilruntime.HandleError(logger.LogError("reconciliation failed: %v", err))
		}
	}
}

func (c *Operator) storageNodeForDeployment(d *extensions.Deployment) *spec.StorageNode {
	key, err := keyFunc(d)
	if err != nil {
		utilruntime.HandleError(logger.LogError("error creating key: %v", err))
		return nil
	}

	// Namespace/Name are one-to-one so the key will find the respective StorageNode resource.
	s, exists, err := c.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		utilruntime.HandleError(logger.LogError("error getting storage node resource: %v", err))
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
		return logger.Err(err)
	}

	// Get plugin
	storage, err := c.GetStorage(s.Spec.Type)
	if err != nil {
		return logger.Err(err)
	}

	obj, exists, err := c.nodeInf.GetStore().GetByKey(key)
	if err != nil {
		return logger.Err(err)
	}

	if !exists {
		err := storage.DeleteNode(s)
		if err != nil {
			return logger.Err(err)
		}

		reaper, err := kubectl.ReaperFor(extensions.Kind("Deployment"), c.kclient)
		if err != nil {
			return logger.Err(err)
		}

		err = reaper.Stop(s.Namespace, s.Name, time.Minute, api.NewDeleteOptions(0))
		if err != nil {
			return logger.Err(err)
		}

		return nil
	}

	// Use the copy in the cache
	s = obj.(*spec.StorageNode)

	// DeepCopy CS
	s, err = storageNodeDeepCopy(s)
	if err != nil {
		return err
	}

	deployClient := c.kclient.Extensions().Deployments(s.Namespace)
	deployment := &extensions.Deployment{}
	deployment.Namespace = s.Namespace
	deployment.Name = s.Name
	obj, exists, err = c.dsetInf.GetStore().Get(deployment)
	if err != nil {
		return logger.Err(err)
	}

	if !exists {

		// Get a deployment from plugin
		deploy, err := storage.MakeDeployment(s, nil)
		if err != nil {
			return logger.Err(err)
		}

		if _, err := deployClient.Create(deploy); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return logger.LogError("unable to create deployment %v: %v",
					deploy.GetName(), err)
			}
		}

		// We can now spawn a new routine which will wait until
		// the deployment is ready.
		go func() {

			// Wait until it is ready
			err = WaitForDeploymentReady(c.kclient,
				s.GetNamespace(),
				s.GetName(),
				deploy.Spec.Replicas)
			if err != nil {
				logger.Err(err)
				return
			}

			// Add node
			updated, err := storage.AddNode(s)
			if err != nil {
				logger.Err(err)
				return
			}

			// Update node object
			if updated != nil {
				storagenodes := qmclient.NewStorageNodes(c.rclient, s.GetNamespace())
				_, err = storagenodes.Update(updated)
				if err != nil {
					logger.Err(err)
					return
				}
			}
		}()

	} else {
		// Update
		deploy, err := storage.MakeDeployment(s, obj.(*extensions.Deployment))
		if err != nil {
			return logger.Err(err)
		}

		// TODO(barakmich): This may be broken for DaemonSets.
		// Will be fixed when DaemonSets do rolling updates.
		if _, err := deployClient.Update(deploy); err != nil {
			return logger.Err(err)
		}

		// Update Node
		_, err = storage.UpdateNode(s)
		if err != nil {
			return logger.Err(err)
		}
	}

	return nil
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

func storageNodeDeepCopy(ns *spec.StorageNode) (*spec.StorageNode, error) {
	objCopy, err := api.Scheme.DeepCopy(ns)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*spec.StorageNode)
	if !ok {
		return nil, logger.LogError("expected StorageNode, got %#v", objCopy)
	}
	return copied, nil
}
