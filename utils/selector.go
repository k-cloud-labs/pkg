package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

// ResourceMatchSelectors tells if the specific resource matches the selectors.
func ResourceMatchSelectors(resource *unstructured.Unstructured, selectors ...policyv1alpha1.ResourceSelector) bool {
	for _, rs := range selectors {
		if ResourceMatches(resource, rs) {
			return true
		}
	}
	return false
}

// ResourceMatches tells if the specific resource matches the selector.
func ResourceMatches(resource *unstructured.Unstructured, rs policyv1alpha1.ResourceSelector) bool {
	if resource.GetAPIVersion() != rs.APIVersion ||
		resource.GetKind() != rs.Kind ||
		(len(rs.Namespace) > 0 && resource.GetNamespace() != rs.Namespace) {
		return false
	}

	/*
		* match rules:
		* (any means no matter if it's empty or not)
		| name | label selector | field selector | result |
		|:---- |:----          |:----          |:----   |
		| not empty | any       | any       | match name only |
		| empty     | empty     | empty     | match all |
		| empty     | not empty | empty     | match labels only |
		| empty     | empty     | not empty | match fields only |
		| empty     | not empty | not empty | match both labels and fields |
	*/

	// name not empty, don't need to consult selector.
	if len(rs.Name) > 0 {
		return rs.Name == resource.GetName()
	}

	// all empty, matches all
	if rs.LabelSelector == nil && rs.FieldSelector == nil {
		return true
	}

	// matches with field selector
	if rs.FieldSelector != nil {
		match, err := rs.FieldSelector.MatchObject(resource)
		if err != nil {
			klog.ErrorS(err, "match fields failed")
			return false
		}

		if !match {
			// return false if not match
			return false
		}
	}

	// matches with selector
	if rs.LabelSelector != nil {
		var s labels.Selector
		var err error
		if s, err = metav1.LabelSelectorAsSelector(rs.LabelSelector); err != nil {
			// should not happen because all resource selector should be fully validated by webhook.
			klog.ErrorS(err, "match labels failed")
			return false
		}

		return s.Matches(labels.Set(resource.GetLabels()))
	}

	return true
}
