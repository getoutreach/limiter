package main

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	annotationName = "outreach.io/reconcile-limits"
	limitRangeName = "limits"
)

// Controller dynamically injects (and subsequently reconciles) LimitRange resources in each namespace.
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	limitRangeLister  corev1listers.LimitRangeLister
	limitRangesSynced cache.InformerSynced
	namespaceLister   corev1listers.NamespaceLister
	namespacesSynced  cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

// NewController returns a new Controller
func NewController(kubeclientset kubernetes.Interface, kubeInformerFactory informers.SharedInformerFactory) *Controller {

	// obtain reference to shared index informer
	limitRangeInformer := kubeInformerFactory.Core().V1().LimitRanges()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()

	controller := &Controller{
		kubeclientset: kubeclientset,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "namespaces"),

		limitRangeLister:  limitRangeInformer.Lister(),
		limitRangesSynced: limitRangeInformer.Informer().HasSynced,
		namespaceLister:   namespaceInformer.Lister(),
		namespacesSynced:  namespaceInformer.Informer().HasSynced,
	}

	glog.Info("Setting up event handlers")

	// Set up an event handler for when Namespace resources change
	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueNamespace,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueNamespace(new)
		},
	})

	// Set up an event handler for when LimitRange resources change.
	// Assuming the namespace-level `outreach.io/override-limits` annotation is absent,
	// we clobber changes / enforce the defaults by enqueuing the parent namespace.
	// This way, we don't need to implement custom logic for handling LimitRange resources.
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	limitRangeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newLimits := new.(*corev1.LimitRange)
			oldLimits := old.(*corev1.LimitRange)
			if newLimits.ResourceVersion == oldLimits.ResourceVersion {
				// Periodic resync will send update events for all known LimitRanges.
				// Two different versions of the same LimitRanges will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Namespace controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.namespacesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch workers to process Namespace resources
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
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {

	ns, err := c.namespaceLister.Get(key)
	if err != nil {
		// The Namespace resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("namespace '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// TODO: allow additional exclusions, configurable at runtime
	// Verify this isn't a special namespace before proceeding
	if ns.GetName() == "kube-system" {
		return nil
	}

	annotations := ns.GetAnnotations()
	reconcile, ok := annotations[annotationName]
	if !ok || (reconcile != "false") {
		glog.V(4).Infof("Processing object: %s/%s", ns.GetName(), limitRangeName)

		// Check if LimitRange exists
		_, err = c.limitRangeLister.LimitRanges(ns.Name).Get(limitRangeName)

		if errors.IsNotFound(err) { // Create LimitRange
			_, err = c.kubeclientset.CoreV1().LimitRanges(ns.Name).Create(newLimitRange(ns))
		} else if err == nil {
			_, err = c.kubeclientset.CoreV1().LimitRanges(ns.Name).Update(newLimitRange(ns))
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// enqueueNamespace takes a Namespace resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Namespace.
func (c *Controller) enqueueNamespace(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object.
// It fetches (and enqueues) the parent Namespace resource to be processed
// unless metadata.annotations["outreach.io/reconcile-limits"] = false.
func (c *Controller) handleObject(obj interface{}) {
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

	ns, err := c.namespaceLister.Get(object.GetNamespace())
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.enqueueNamespace(ns)
}

// newLimitRange creates a new LimitRange for a Namespace resource.
func newLimitRange(ns *corev1.Namespace) *corev1.LimitRange {

	containerLimits := corev1.LimitRangeItem{
		Default: corev1.ResourceList{
			"memory": resource.MustParse(config.container.limit.memory),
		},
		DefaultRequest: corev1.ResourceList{
			"cpu":    resource.MustParse(config.container.request.cpu),
			"memory": resource.MustParse(config.container.request.memory),
		},
		Max:  corev1.ResourceList{},
		Min:  corev1.ResourceList{},
		Type: "Container",
	}
	podLimits := corev1.LimitRangeItem{
		Default:        corev1.ResourceList{},
		DefaultRequest: corev1.ResourceList{},
		Max:            corev1.ResourceList{},
		Min:            corev1.ResourceList{},
		Type:           "Pod",
	}

	if config.container.limit.cpu != "" {
		containerLimits.Default["cpu"] = resource.MustParse(config.container.limit.cpu)
	}

	if config.container.max.cpu != "" {
		containerLimits.Max["cpu"] = resource.MustParse(config.container.max.cpu)
	}

	if config.container.max.memory != "" {
		containerLimits.Max["memory"] = resource.MustParse(config.container.max.memory)
	}

	if config.container.min.cpu != "" {
		containerLimits.Min["cpu"] = resource.MustParse(config.container.min.cpu)
	}

	if config.container.min.memory != "" {
		containerLimits.Min["memory"] = resource.MustParse(config.container.min.memory)
	}

	if config.pod.max.cpu != "" {
		podLimits.Max["cpu"] = resource.MustParse(config.pod.max.cpu)
	}

	if config.pod.max.memory != "" {
		podLimits.Max["memory"] = resource.MustParse(config.pod.max.memory)
	}

	if config.pod.min.cpu != "" {
		podLimits.Min["cpu"] = resource.MustParse(config.pod.min.cpu)
	}

	if config.pod.min.memory != "" {
		podLimits.Min["memory"] = resource.MustParse(config.pod.min.memory)
	}

	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "limits",
			Namespace: ns.Name,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{containerLimits, podLimits},
		},
	}

}
