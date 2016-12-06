// Copyright 2016 The prometheus-operator Authors
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	apierrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/labels"
	utilruntime "k8s.io/client-go/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	TPRGroup   = "storage.coreos.com"
	TPRVersion = "v1alpha1"

	TPRStorageNodeKind   = "storagenodes"
	TPRStorageStatusKind = "storagestatuses"

	tprStorageNode   = "nodes." + TPRGroup
	tprStorageStatus = "status." + TPRGroup
)

// Operator manages lify cycle of Prometheus deployments and
// monitoring configurations.
type Operator struct {
	kclient *kubernetes.Clientset
	rclient *rest.RESTClient
	logger  log.Logger

	storage storageType

	nodeInf cache.SharedIndexInformer
	dsetInf cache.SharedIndexInformer

	queue *queue

	host string
}

// Config defines configuration parameters for the Operator.
type Config struct {
	Host        string
	TLSInsecure bool
	TLSConfig   rest.TLSClientConfig
}

// New creates a new controller.
func New(c Config) (*Operator, error) {
	cfg, err := newClusterConfig(c.Host, c.TLSInsecure, &c.TLSConfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	logger := log.NewContext(log.NewLogfmtLogger(os.Stdout)).
		With("ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	rclient, err := newQuartermasterRESTClient(*cfg)
	if err != nil {
		return nil, err
	}
	return &Operator{
		kclient: client,
		rclient: rclient,
		logger:  logger,
		queue:   newQueue(200),
		host:    cfg.Host,
	}, nil
}

// Run the controller.
func (c *Operator) Run(stopc <-chan struct{}) error {
	defer c.queue.close()
	go c.worker()

	v, err := c.kclient.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("communicating with server failed: %s", err)
	}
	c.logger.Log("msg", "connection established", "cluster-version", v)

	if err := c.createTPRs(); err != nil {
		return err
	}

	c.nodeInf = cache.NewSharedIndexInformer(
		NewStorageNodeListWatch(c.rclient),
		&spec.StorageNode{}, resyncPeriod, cache.Indexers{},
	)
	c.dsetInf = cache.NewSharedIndexInformer(
		cache.NewListWatchFromClient(c.kclient.Apps().RESTClient(), "daemonsets", api.NamespaceAll, nil),
		&v1beta1.DaemonSet{}, resyncPeriod, cache.Indexers{},
	)

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
			c.logger.Log("msg", "addDaemonSet", "trigger", "ds add")
			c.addDaemonSet(d)
		},
		DeleteFunc: func(d interface{}) {
			c.logger.Log("msg", "deleteDaemonSet", "trigger", "ds delete")
			c.deleteDaemonSet(d)
		},
		UpdateFunc: func(old, cur interface{}) {
			c.logger.Log("msg", "updateDaemonSet", "trigger", "ds update")
			c.updateDaemonSet(old, cur)
		},
	})

	go c.nodeInf.Run(stopc)
	go c.dsetInf.Run(stopc)

	for !c.nodeInf.HasSynced() || !c.dsetInf.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}

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

func (c *Operator) storageNodeForDeployment(d *v1beta1.DaemonSet) *spec.StorageNode {
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

func (c *Operator) deleteDaemonSet(o interface{}) {
	d := o.(*v1beta1.DaemonSet)
	if s := c.storageNodeForDeployment(d); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *Operator) addDaemonSet(o interface{}) {
	d := o.(*v1beta1.DaemonSet)
	if s := c.storageNodeForDeployment(d); s != nil {
		c.enqueueStorageNode(s)
	}
}

func (c *Operator) updateDaemonSet(oldo, curo interface{}) {
	old := oldo.(*v1beta1.DaemonSet)
	cur := curo.(*v1beta1.DaemonSet)

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
	if !exists {
		// TODO(fabxc): we want to do server side deletion due to the variety of
		// resources we create.
		// Doing so just based on the deletion event is not reliable, so
		// we have to garbage collect the controller-created resources in some other way.
		//
		// Let's rely on the index key matching that of the created configmap and replica
		// set for now. This does not work if we delete Prometheus resources as the
		// controller is not running – that could be solved via garbage collection later.
		return c.deleteStorageNode(s)
	}

	dsetClient := c.kclient.ExtensionsV1beta1().DaemonSets(s.Namespace)
	// Ensure we have a replica set running Prometheus deployed.
	// XXX: Selecting by ObjectMeta.Name gives an error. So use the label for now.
	psetQ := &v1beta1.DaemonSet{}
	psetQ.Namespace = s.Namespace
	psetQ.Name = s.Name
	obj, exists, err := c.dsetInf.GetStore().Get(psetQ)
	if err != nil {
		return err
	}

	if !exists {
		ds, err := c.storage.MakeDaemonSet(s, nil)
		if err != nil {
			return err
		}
		if _, err := dsetClient.Create(ds); err != nil {
			return fmt.Errorf("create daemonset: %s", err)
		}
	} else {
		// Update
		ds, err := c.storage.MakeDaemonSet(s, obj.(*v1beta1.DaemonSet))
		if err != nil {
			return err
		}
		// TODO(barakmich): This may be broken for DaemonSets.
		// Will be fixed when DaemonSets do rolling updates.
		if _, err := dsetClient.Update(ds); err != nil {
			return err
		}
	}

	return c.updateStatus()
}

func (c *Operator) updateStatus() error {
	// TODO(barakmich): Call into c.storage.GetStatus()
	return nil
}

func ListOptions(name string) v1.ListOptions {
	s := labels.SelectorFromSet(map[string]string{
		"app":           "quartermaster",
		"quartermaster": name,
	})
	return v1.ListOptions{
		LabelSelector: s.String(),
	}
}

func (c *Operator) deleteStorageNode(s *spec.StorageNode) error {
	// Update the replica count to 0 and wait for all pods to be deleted.
	dsetClient := c.kclient.ExtensionsV1beta1().DaemonSets(s.Namespace)

	return dsetClient.DeleteCollection(nil, ListOptions(s.Name))
}

func (c *Operator) createTPRs() error {
	tprs := []*v1beta1.ThirdPartyResource{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: tprStorageStatus,
			},
			Versions: []v1beta1.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Status reports from Quartermaster managed storage",
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: tprStorageNode,
			},
			Versions: []v1beta1.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Managed storage nodes via Quartermaster",
		},
	}
	tprClient := c.kclient.Extensions().ThirdPartyResources()

	for _, tpr := range tprs {
		if _, err := tprClient.Create(tpr); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		c.logger.Log("msg", "TPR created", "tpr", tpr.Name)
	}

	// We have to wait for the TPRs to be ready. Otherwise the initial watch may fail.
	err := WaitForTPRReady(c.kclient.CoreV1Client.RESTClient(), TPRGroup, TPRVersion, TPRStorageNodeKind)
	if err != nil {
		return err
	}
	return WaitForTPRReady(c.kclient.CoreV1Client.RESTClient(), TPRGroup, TPRVersion, TPRStorageStatusKind)
}

func newClusterConfig(host string, tlsInsecure bool, tlsConfig *rest.TLSClientConfig) (*rest.Config, error) {
	var cfg *rest.Config
	var err error

	if len(host) == 0 {
		if cfg, err = rest.InClusterConfig(); err != nil {
			return nil, err
		}
	} else {
		cfg = &rest.Config{
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
