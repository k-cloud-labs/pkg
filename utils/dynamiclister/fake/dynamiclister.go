package fake

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	fakedDynamic "k8s.io/client-go/dynamic/fake"
	fakedK8s "k8s.io/client-go/kubernetes/fake"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
	"github.com/k-cloud-labs/pkg/utils/informermanager"
)

type FakeResourceListerImpl struct {
	dynamicInterface dynamic.Interface
	informer         informermanager.SingleClusterInformerManager
	listerMap        sync.Map // gvk:lister
	mapper           meta.RESTMapper
}

var (
	_ dynamiclister.DynamicResourceLister = &FakeResourceListerImpl{}
)

func NewFakeDynamicResourceLister(done <-chan struct{}, objects ...runtime.Object) (*FakeResourceListerImpl, error) {
	d := &FakeResourceListerImpl{
		listerMap: sync.Map{},
	}

	cs := fakedK8s.NewSimpleClientset(objects...).Discovery()
	dc := fakedDynamic.NewSimpleDynamicClient(k8sScheme.Scheme, objects...)
	d.dynamicInterface = dc
	d.informer = informermanager.NewSingleClusterInformerManager(dc, 0, done)

	rs, err := restmapper.GetAPIGroupResources(cs)
	if err != nil {
		return nil, err
	}
	klog.InfoS("rs", "len", len(rs))
	d.mapper = restmapper.NewDiscoveryRESTMapper(rs)

	return d, nil
}

func (d *FakeResourceListerImpl) RegisterNewResource(gvk schema.GroupVersionKind, waitForSync bool) bool {
	gvr, err := d.gvk2Gvr(gvk)
	if err != nil {
		return false
	}

	_, ok := d.listerMap.Load(gvr.String())
	if ok {
		return true
	}

	// not found try to create new lister
	lister := d.informer.Lister(gvr)
	d.informer.Start()
	if waitForSync {
		d.informer.WaitForCacheSync()
	}
	d.listerMap.Store(gvr.String(), lister)
	return true
}

func (d *FakeResourceListerImpl) GVKToResourceLister(gvk schema.GroupVersionKind) (cache.GenericLister, error) {
	gvr, err := d.gvk2Gvr(gvk)
	if err != nil {
		return nil, err
	}

	v, ok := d.listerMap.Load(gvr.String())
	if ok {
		klog.Info("loaded exist lister")
		return v.(cache.GenericLister), nil
	}

	return &simpleLister{
		gvr: gvr,
		di:  d.dynamicInterface,
	}, nil
}

func (d *FakeResourceListerImpl) gvk2Gvr(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	rm, err := d.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		klog.ErrorS(err, "RESTMapping")
		rm = &meta.RESTMapping{
			Resource: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments", // for test
			},
		}
	}

	klog.InfoS("RESTMapping", "gvk", rm.Resource.String())
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
