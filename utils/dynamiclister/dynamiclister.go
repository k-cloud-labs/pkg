package dynamiclister

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/k-cloud-labs/pkg/utils/informermanager"
)

// DynamicResourceLister define a cached dynamic resource lister
type DynamicResourceLister interface {
	// RegisterNewResource add new type of gvr to cache and sync to cache.
	// If second param waitForSync is true, then it will wait for sync data finish.
	// It returns true if gvk exist in mem cache or add success, otherwise return false.
	RegisterNewResource(gvk schema.GroupVersionKind, waitForSync bool) bool
	// GVKToResourceLister try load resource lister from local cache, if not found in local then request
	// k8s api to get resource.
	GVKToResourceLister(schema.GroupVersionKind) (cache.GenericLister, error)
}

// dynamicResourceListerImpl is implement of DynamicResourceLister
type dynamicResourceListerImpl struct {
	dynamicInterface dynamic.Interface
	informer         informermanager.SingleClusterInformerManager
	listerMap        sync.Map // gvk:lister
	mapper           *restmapper.DeferredDiscoveryRESTMapper
	gvkToGVRMap      sync.Map
}

// NewDynamicResourceLister init DynamicResourceLister implemented by dynamicResourceListerImpl.
func NewDynamicResourceLister(cfg *rest.Config, done <-chan struct{}, gvrList ...schema.GroupVersionResource) (DynamicResourceLister, error) {
	d := &dynamicResourceListerImpl{
		informer:         informermanager.NewSingleClusterInformerManager(dynamic.NewForConfigOrDie(cfg), 0, done),
		listerMap:        sync.Map{},
		dynamicInterface: dynamic.NewForConfigOrDie(cfg),
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	d.mapper = restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	for _, gvr := range gvrList {
		lister := d.informer.Lister(gvr)
		d.informer.Start()
		if m := d.informer.WaitForCacheSync(); !m[gvr] {
			return nil, fmt.Errorf("sync resource(%v) failed", gvr.String())
		}
		d.listerMap.Store(gvr.String(), lister)
	}

	return d, nil
}

func (d *dynamicResourceListerImpl) RegisterNewResource(gvk schema.GroupVersionKind, waitForSync bool) bool {
	_, ok := d.listerMap.Load(gvk.String())
	if ok {
		return true
	}

	gvr, err := d.gvk2Gvr(gvk)
	if err != nil {
		return false
	}

	// not found try to create new lister
	lister := d.informer.Lister(gvr)
	d.informer.Start()
	if waitForSync {
		d.informer.WaitForCacheSync()
	}
	d.listerMap.Store(gvk.String(), lister)
	return true
}

func (d *dynamicResourceListerImpl) GVKToResourceLister(gvk schema.GroupVersionKind) (cache.GenericLister, error) {
	v, ok := d.listerMap.Load(gvk.String())
	if ok {
		klog.Info("loaded exist lister")
		return v.(cache.GenericLister), nil
	}

	gvr, err := d.gvk2Gvr(gvk)
	if err != nil {
		return nil, err
	}

	return &simpleLister{
		gvr: gvr,
		di:  d.dynamicInterface,
	}, nil
}

func (d *dynamicResourceListerImpl) gvk2Gvr(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	v, ok := d.gvkToGVRMap.Load(gvk.String())
	if ok {
		return v.(schema.GroupVersionResource), nil
	}

	rm, err := d.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	d.gvkToGVRMap.Store(gvk.String(), rm.Resource)
	return rm.Resource, nil
}

type simpleLister struct {
	namespace string
	gvr       schema.GroupVersionResource
	di        dynamic.Interface
}

func (s *simpleLister) List(selector labels.Selector) (result []runtime.Object, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var list *unstructured.UnstructuredList
	if s.namespace != "" {
		list, err = s.di.Resource(s.gvr).Namespace(s.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
			Limit:         1,
		})
	} else {
		list, err = s.di.Resource(s.gvr).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
			Limit:         1,
		})
	}
	if err != nil {
		return nil, err
	}

	if list == nil || len(list.Items) == 0 {
		return nil, nil
	}

	result = []runtime.Object{
		&list.Items[0],
	}

	return
}

func (s *simpleLister) Get(name string) (runtime.Object, error) {
	klog.Info("from simple lister")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if s.namespace != "" {
		return s.di.Resource(s.gvr).Namespace(s.namespace).Get(ctx, name, metav1.GetOptions{})
	}

	return s.di.Resource(s.gvr).Get(ctx, name, metav1.GetOptions{})
}

func (s *simpleLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	s.namespace = namespace
	return s
}
