package cue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/dynamicclient"
)

type CueParams struct {
	Object    *unstructured.Unstructured `json:"object"`
	OldObject *unstructured.Unstructured `json:"oldObject"`
	// otherObject:xxx, http:xxx
	ExtraParams map[string]any `json:"extraParams"`
}

func BuildCueParamsViaOverridePolicy(c dynamicclient.IDynamicClient, curObject *unstructured.Unstructured, tmpl *policyv1alpha1.OverrideRuleTemplate) (*CueParams, error) {
	var (
		cp = &CueParams{
			ExtraParams: make(map[string]any),
		}
	)
	if tmpl.ValueRef != nil {
		klog.V(2).InfoS("BuildCueParamsViaOverridePolicy value ref", "refFrom", tmpl.ValueRef.From)
		if tmpl.ValueRef.From == policyv1alpha1.FromOwnerReference {
			obj, err := getOwnerReference(c, curObject)
			if err != nil {
				return nil, fmt.Errorf("getOwnerReference got error=%w", err)
			}
			cp.ExtraParams["otherObject"] = obj
		}
		if tmpl.ValueRef.From == policyv1alpha1.FromK8s {
			obj, err := getObject(c, curObject, tmpl.ValueRef.K8s)
			if err != nil {
				return nil, fmt.Errorf("getObject got error=%w", err)
			}
			cp.ExtraParams["otherObject"] = obj
		}

		if tmpl.ValueRef.From == policyv1alpha1.FromHTTP {
			obj, err := getHttpResponse(nil, curObject, tmpl.ValueRef.Http)
			if err != nil {
				return nil, fmt.Errorf("getHttpResponse got error=%w", err)
			}
			cp.ExtraParams["http"] = obj
		}
	}

	return cp, nil
}

func BuildCueParamsViaValidatePolicy(c dynamicclient.IDynamicClient, curObject *unstructured.Unstructured, tmpl *policyv1alpha1.ValidateRuleTemplate) (*CueParams, error) {
	switch tmpl.Type {
	case policyv1alpha1.ValidateRuleTypeCondition:
		return buildCueParamsForValidateCondition(c, curObject, tmpl.Condition)
	case policyv1alpha1.ValidateRuleTypePodAvailableBadge:
		return buildCueParamsForPAB(c, curObject, tmpl.PodAvailableBadge)
	default:
		return nil, fmt.Errorf("unknown template type(%v)", tmpl.Type)
	}
}

func buildCueParamsForValidateCondition(c dynamicclient.IDynamicClient, curObject *unstructured.Unstructured, condition *policyv1alpha1.ValidateCondition) (*CueParams, error) {
	var cp = &CueParams{
		ExtraParams: make(map[string]any),
	}

	if condition.ValueRef != nil {
		if condition.ValueRef.From == policyv1alpha1.FromOwnerReference {
			obj, err := getOwnerReference(c, curObject)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["otherObject"] = obj
		}
		if condition.ValueRef.From == policyv1alpha1.FromK8s {
			obj, err := getObject(c, curObject, condition.ValueRef.K8s)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["otherObject"] = obj
		}

		if condition.ValueRef.From == policyv1alpha1.FromHTTP {
			obj, err := getHttpResponse(nil, curObject, condition.ValueRef.Http)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["http"] = obj
		}
	}

	if condition.DataRef != nil {
		if condition.DataRef.From == policyv1alpha1.FromOwnerReference {
			obj, err := getOwnerReference(c, curObject)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["otherObject_d"] = obj
		}
		if condition.DataRef.From == policyv1alpha1.FromK8s {
			obj, err := getObject(c, curObject, condition.DataRef.K8s)
			if err != nil {
				return nil, err
			}
			// _d for dataRef
			cp.ExtraParams["otherObject_d"] = obj
		}

		if condition.DataRef.From == policyv1alpha1.FromHTTP {
			obj, err := getHttpResponse(nil, curObject, condition.DataRef.Http)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["http_d"] = obj
		}
	}

	return cp, nil
}

func buildCueParamsForPAB(c dynamicclient.IDynamicClient, curObject *unstructured.Unstructured, pab *policyv1alpha1.PodAvailableBadge) (*CueParams, error) {
	var cp = &CueParams{
		ExtraParams: make(map[string]any),
	}

	if pab.ReplicaReference == nil || pab.ReplicaReference.From == policyv1alpha1.FromOwnerReference {
		// get owner reference in default case
		obj, err := getOwnerReference(c, curObject)
		if err != nil {
			return nil, fmt.Errorf("getOwnerReference got error=%w", err)
		}
		cp.ExtraParams["otherObject"] = obj
		return cp, nil
	}

	if pab.ReplicaReference.From == policyv1alpha1.FromK8s {
		obj, err := getObject(c, curObject, pab.ReplicaReference.K8s)
		if err != nil {
			return nil, fmt.Errorf("getObject got error=%w", err)
		}
		cp.ExtraParams["otherObject"] = obj
	}

	if pab.ReplicaReference.From == policyv1alpha1.FromHTTP {
		obj, err := getHttpResponse(nil, curObject, pab.ReplicaReference.Http)
		if err != nil {
			return nil, fmt.Errorf("getHttpResponse got error=%w", err)
		}
		cp.ExtraParams["http"] = obj
	}

	return cp, nil
}

func getObject(c dynamicclient.IDynamicClient, obj *unstructured.Unstructured, rs *policyv1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	gvk := schema.FromAPIVersionAndKind(rs.APIVersion, rs.Kind)

	rc, err := c.GetResourceClientByGVK(gvk)
	if err != nil {
		klog.ErrorS(err, "GetGroupVersionResource got error",
			"apiVersion", rs.APIVersion, "kind", rs.Kind, "name", rs.Name)
		return nil, err
	}

	if rs.Name != "" {
		refName, err := parseAndGetRefValue(rs.Name, obj)
		if err != nil {
			return nil, err
		}
		refNs, err := parseAndGetRefValue(rs.Namespace, obj)
		if err != nil {
			return nil, err
		}

		klog.V(4).InfoS("get owner reference", "apiVersion", rs.APIVersion, "kind", rs.Kind, "name", refName)
		obj, err := rc.Namespace(refNs).Get(ctx, refName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return obj, nil
	}

	if rs.LabelSelector != nil {
		handled, err := handleRefSelectLabels(rs.LabelSelector, obj)
		if err != nil {
			return nil, err
		}
		// use selector
		s, err := metav1.LabelSelectorAsSelector(handled)
		if err != nil {
			return nil, err
		}

		klog.V(4).InfoS("get object", "label", s.String())
		list, err := rc.Namespace(rs.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: s.String(),
		})
		if err != nil {
			return nil, err
		}

		klog.V(4).InfoS("list by label", "list", list.Items, "obj", list.Object)
		if len(list.Items) > 0 {
			return &list.Items[0], nil
		}
	}

	return nil, nil
}

func handleRefSelectLabels(ls *metav1.LabelSelector, obj *unstructured.Unstructured) (*metav1.LabelSelector, error) {
	result := &metav1.LabelSelector{
		MatchLabels:      make(map[string]string),
		MatchExpressions: make([]metav1.LabelSelectorRequirement, 0),
	}
	for k, v := range ls.MatchLabels {
		refVal, err := parseAndGetRefValue(v, obj)
		if err != nil {
			return nil, err
		}

		result.MatchLabels[k] = refVal
	}

	for _, expression := range ls.MatchExpressions {
		var values []string
		for _, value := range expression.Values {
			refVal, err := parseAndGetRefValue(value, obj)
			if err != nil {
				return nil, err
			}

			values = append(values, refVal)
		}

		result.MatchExpressions = append(result.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      expression.Key,
			Operator: expression.Operator,
			Values:   values,
		})
	}

	return result, nil
}

func getOwnerReference(c dynamicclient.IDynamicClient, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	list := obj.GetOwnerReferences()
	if len(list) == 0 {
		return nil, errors.New("object has no owner reference")
	}

	or := list[0]
	gvk := schema.FromAPIVersionAndKind(or.APIVersion, or.Kind)
	klog.V(4).InfoS("get owner reference", "apiVersion", or.APIVersion, "kind", or.Kind, "name", or.Name)

	rc, err := c.GetResourceClientByGVK(gvk)
	if err != nil {
		klog.ErrorS(err, "GetGroupVersionResource got error", "apiVersion", or.APIVersion, "kind", or.Kind, "name", or.Name)
		return nil, err
	}

	return rc.Namespace(obj.GetNamespace()).Get(ctx, or.Name, metav1.GetOptions{})
}

func getHttpResponse(c *http.Client, obj *unstructured.Unstructured, ref *policyv1alpha1.HttpDataRef) (map[string]any, error) {
	if c == nil {
		c = &http.Client{
			Transport: http.DefaultTransport,
			Timeout:   time.Second * 3,
		}
	}

	var (
		query = url.Values{}
		body  io.Reader
	)
	for k, v := range ref.Params {
		refVal, err := parseAndGetRefValue(v, obj)
		if err != nil {
			return nil, err
		}
		query.Set(k, refVal)
	}

	if len(ref.Body.Raw) > 0 {
		body = bytes.NewBuffer(ref.Body.Raw)
	}

	req, err := http.NewRequest(ref.Method, ref.URL+"?"+query.Encode(), body)
	if err != nil {
		return nil, err
	}
	for k, v := range ref.Header {
		req.Header.Add(k, v)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer noErr(resp.Body.Close)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"body":    string(b),
		"header":  resp.Header,
		"trailer": resp.Trailer,
	}, nil
}

func noErr(f func() error) {
	_ = f()
}

func parseAndGetRefValue(refKey string, obj *unstructured.Unstructured) (string, error) {
	if !(strings.HasPrefix(refKey, "{{") && strings.HasSuffix(refKey, "}}")) {
		return refKey, nil // not ref
	}

	key := strings.TrimPrefix(strings.TrimSuffix(refKey, "}}"), "{{")

	v, ok, err := unstructured.NestedString(obj.Object, strings.Split(key, ".")...)
	if err != nil {
		klog.ErrorS(err, "get reference value from current object got error", "key", key, "object", klog.KObj(obj))
		return "", fmt.Errorf("get reference value from current object got error,err=%w", err)
	}

	if !ok {
		klog.ErrorS(err, "get reference value from current object is not string", "key", key, "object", klog.KObj(obj))
		return "", errors.New("get reference value from current object is not string")
	}

	return v, nil
}
