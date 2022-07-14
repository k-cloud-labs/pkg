package controller_runtime

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type labelIndexerManager struct {
	manager.Manager
}

// NewManager initialize a custom controller runtime manager with specified options to
// add index for specified objects which convert labelSelector to fieldSelector because of
// controller runtime NOT support add index for labelSelector.
// Be care of only single fieldSelector is supported for now when to call List func which is limited by client-go's implementation.
// Refer to: https://github.com/kubernetes/kubernetes/pull/109334
func NewManager(config *rest.Config, options manager.Options, objects []client.Object) (manager.Manager, error) {
	manager, err := manager.New(config, options)
	if err != nil {
		return nil, err
	}

	lim := &labelIndexerManager{
		Manager: manager,
	}

	ctx := context.Background()
	for _, object := range objects {
		err := lim.Manager.GetFieldIndexer().IndexField(ctx, object, "metadata.labels", indexObject)
		if err != nil {
			return nil, err
		}
	}

	return lim, nil
}

func indexObject(obj client.Object) []string {
	val := make([]string, 0)
	for k, v := range obj.GetLabels() {
		val = append(val, k+"/"+v)
	}

	return val
}
