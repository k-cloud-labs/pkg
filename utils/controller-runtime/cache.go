package controller_runtime

import (
	"context"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type labelIndexerCache struct {
	cache.Cache
}

// NewCache initialize a custom cache used to auto convert labelSelector to fieldSelector when call List func.
// Use this func with NewManager the same time, otherwise it will not take effect.
func NewCache(config *rest.Config, opts cache.Options) (cache.Cache, error) {
	cache, err := cache.New(config, opts)
	if err != nil {
		return nil, err
	}

	lic := &labelIndexerCache{
		Cache: cache,
	}

	return lic, err
}

func (lic *labelIndexerCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	// convert labelSelector to fieldSelector
	selectors := make([]labels.Selector, 0)
	for _, opt := range opts {
		listOption, ok := opt.(*client.ListOptions)
		if ok {
			if listOption.LabelSelector != nil && !listOption.LabelSelector.Empty() {
				selectors = append(selectors, listOption.LabelSelector)
			}
		}
	}

	fieldSelectors := make([]fields.Selector, 0)
	indexerKeys := make(map[string]struct{})
	for _, selector := range selectors {
		requirements, _ := selector.Requirements()
		for _, requirement := range requirements {
			if requirement.Operator() != selection.Equals {
				continue
			}
			for v := range requirement.Values() {
				key := requirement.Key() + "/" + v
				if _, ok := indexerKeys[key]; !ok {
					fieldSelectors = append(fieldSelectors, fields.OneTermEqualSelector("metadata.labels", key))
					indexerKeys[key] = struct{}{}
				}
			}
		}
	}

	if len(selectors) > 0 {
		opts = append(opts, &client.ListOptions{FieldSelector: fields.AndSelectors(fieldSelectors...)})
	}

	return lic.Cache.List(ctx, list, opts...)
}
