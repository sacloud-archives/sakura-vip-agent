package main

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const controllerAgentName = "sakura-vip-agent"

const (
	// SuccessSynced is used as part of the Event 'reason' when a VIP is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a VIP
	// is synced successfully
	MessageResourceSynced = "VIP synced successfully"

	actionAdd = "VIP-Add"
	actionDel = "VIP-Del"
)

type workItem struct {
	action  string
	key     string
	service *corev1.Service
}

// Controller is the controller implementation for SAKURA Cloud LoadBalancer VIP resources
type Controller struct {
	kubeClientSet kubernetes.Interface
	serviceLister corelisters.ServiceLister
	serviceSynced cache.InformerSynced
	workQueue     workqueue.RateLimitingInterface
	recorder      record.EventRecorder
}

// NewController returns a new lb-vip controller
func NewController(
	kubeclientset kubernetes.Interface,
	kubeInformerFactory informers.SharedInformerFactory) *Controller {

	serviceInformer := kubeInformerFactory.Core().V1().Services()

	// Create event broadcaster
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeClientSet: kubeclientset,
		serviceLister: serviceInformer.Lister(),
		serviceSynced: serviceInformer.Informer().HasSynced,
		workQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AddVIP"),
		recorder:      recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Service resources change.
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleAdd,
		UpdateFunc: func(old, new interface{}) {
			newSvc := new.(*corev1.Service)
			oldSvc := old.(*corev1.Service)
			if newSvc.ResourceVersion == oldSvc.ResourceVersion {
				return
			}
			if len(oldSvc.Status.LoadBalancer.Ingress) > 0 {
				controller.handleDel(old)
			}
			controller.handleAdd(new)
		},
		DeleteFunc: controller.handleDel,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workQueue/delActionQueue
// and wait for workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workQueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting LoadBalancer VIP controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.serviceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// add/del ActionQueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the queue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workQueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer queue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the queue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the queue and attempted again after a back-off
		// period.
		defer c.workQueue.Done(obj)
		var item *workItem
		var ok bool
		// We expect strings to come off the queue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// queue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// queue.
		if item, ok = obj.(*workItem); !ok {
			// As the item in the queue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in queue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(item); err != nil {
			return fmt.Errorf("error syncing '%s': %s", item.key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workQueue.Forget(obj)
		glog.Infof("Successfully %s synced '%s'", item.action, item.key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(item *workItem) error {

	var err error
	service := item.service

	if !c.isNeedHandle(service) {

		return nil
	}

	vips := []string{}
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		vips = append(vips, ingress.IP)
	}

	switch item.action {
	case actionAdd:
		err = c.addVIPs(vips)
	case actionDel:
		err = c.delVIPs(vips)
	}
	if err != nil {
		return err
	}

	c.recorder.Event(service, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) isNeedHandle(service *corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		glog.V(4).Infof("service %s/%s is not a LoadBalancer", service.Namespace, service.Name)
		return false
	}

	if len(service.Status.LoadBalancer.Ingress) == 0 {
		glog.V(4).Infof("service %s/%s has no External IP yet", service.Namespace, service.Name)
		return false
	}

	return true
}

func (c *Controller) handleObject(obj interface{}, action string) {

	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	glog.V(4).Infof("Processing object(%s): %s", action, object.GetName())
	if service, ok := object.(*corev1.Service); ok {
		glog.V(4).Infof("Processing target service: %#v", service)
		c.enqueueAction(service, action)
		return
	}
}

func (c *Controller) handleAdd(obj interface{}) {
	c.handleObject(obj, actionAdd)
}

func (c *Controller) handleDel(obj interface{}) {
	c.handleObject(obj, actionDel)
}

func (c *Controller) enqueueAction(service *corev1.Service, action string) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(service); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workQueue.AddRateLimited(&workItem{
		key:     key,
		action:  action,
		service: service,
	})
}
