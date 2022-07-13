package restmapper

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var gvkToGVRMap sync.Map

// GetGroupVersionResource is a helper to map GVK(schema.GroupVersionKind) to GVR(schema.GroupVersionResource).
// Call this func when you need to get gvr by same gvk many times.
// Refer to: https://github.com/karmada-io/karmada/issues/2105
func GetGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	value, ok := gvkToGVRMap.Load(gvk)
	if !ok {
		restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return schema.GroupVersionResource{}, err
		}

		gvkToGVRMap.Store(gvk, restMapping.Resource)
		value = restMapping.Resource
	}

	return value.(schema.GroupVersionResource), nil
}
