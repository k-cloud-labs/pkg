package fake

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	fakedDynamic "k8s.io/client-go/dynamic/fake"
	fakedK8s "k8s.io/client-go/kubernetes/fake"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"

	"github.com/k-cloud-labs/pkg/utils/dynamicclient"
)

// FakeDynamicClient dynamic client for testing
type FakeDynamicClient struct {
	dynamicInterface dynamic.Interface
	mapper           meta.RESTMapper
}

var (
	_ dynamicclient.IDynamicClient = &FakeDynamicClient{}
)

func NewSimpleDynamicClient(objects ...runtime.Object) (*FakeDynamicClient, error) {
	cs := fakedK8s.NewSimpleClientset(objects...).Discovery()
	dc := fakedDynamic.NewSimpleDynamicClient(k8sScheme.Scheme, objects...)

	rs, err := restmapper.GetAPIGroupResources(cs)
	if err != nil {
		return nil, err
	}

	klog.InfoS("rs", "len", len(rs))

	return &FakeDynamicClient{
		dynamicInterface: dc,
		mapper:           restmapper.NewDiscoveryRESTMapper(rs),
	}, nil
}

// GetResourceClientByGVK convert gvk to gvr and return resource interface
func (c *FakeDynamicClient) GetResourceClientByGVK(gvk schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, error) {
	klog.InfoS("GetResourceClientByGVK", "gvk", gvk.String())
	rm, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
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
	return c.dynamicInterface.Resource(rm.Resource), nil
}
