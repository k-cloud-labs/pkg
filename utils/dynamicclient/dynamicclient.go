package dynamicclient

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type IDynamicClient interface {
	GetResourceClientByGVK(schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, error)
}

type DynamicClient struct {
	dynamicInterface dynamic.Interface
	mapper           *restmapper.DeferredDiscoveryRESTMapper
	gvkToGVRMap      sync.Map
}

// NewDynamicClient create dynamic client
func NewDynamicClient(cfg *rest.Config) (*DynamicClient, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	mapperGVRGVK := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	c := dynamic.NewForConfigOrDie(cfg)

	return &DynamicClient{
		dynamicInterface: c,
		mapper:           mapperGVRGVK,
		gvkToGVRMap:      sync.Map{},
	}, nil
}

// GetResourceClientByGVK convert gvk to gvr and return resource interface
func (c *DynamicClient) GetResourceClientByGVK(gvk schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, error) {
	gvr, err := c.GetGroupVersionResource(gvk)
	if err != nil {
		return nil, err
	}

	return c.dynamicInterface.Resource(gvr), nil
}

// GetGroupVersionResource load gvr from cache or get from mapping and store in cache
func (c *DynamicClient) GetGroupVersionResource(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	v, ok := c.gvkToGVRMap.Load(gvk.String())
	if ok {
		return v.(schema.GroupVersionResource), nil
	}

	rm, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	c.gvkToGVRMap.Store(gvk.String(), rm.Resource)
	return rm.Resource, nil
}
