package cue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/dynamiclister"
)

type CueParams struct {
	Object    *unstructured.Unstructured `json:"object"`
	OldObject *unstructured.Unstructured `json:"oldObject"`
	// otherObject:xxx, http:xxx
	ExtraParams map[string]any `json:"extraParams"`
}

func BuildCueParamsViaOverridePolicy(c dynamiclister.DynamicResourceLister, curObject *unstructured.Unstructured, tmpl *policyv1alpha1.OverrideRuleTemplate) (*CueParams, error) {
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

func BuildCueParamsViaValidatePolicy(c dynamiclister.DynamicResourceLister, curObject *unstructured.Unstructured, tmpl *policyv1alpha1.ValidateRuleTemplate) (*CueParams, error) {
	switch tmpl.Type {
	case policyv1alpha1.ValidateRuleTypeCondition:
		return buildCueParamsForValidateCondition(c, curObject, tmpl.Condition)
	case policyv1alpha1.ValidateRuleTypePodAvailableBadge:
		return buildCueParamsForPAB(c, curObject, tmpl.PodAvailableBadge)
	default:
		return nil, fmt.Errorf("unknown template type(%v)", tmpl.Type)
	}
}

func buildCueParamsForValidateCondition(c dynamiclister.DynamicResourceLister, curObject *unstructured.Unstructured, condition *policyv1alpha1.ValidateCondition) (*CueParams, error) {
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

func buildCueParamsForPAB(c dynamiclister.DynamicResourceLister, curObject *unstructured.Unstructured, pab *policyv1alpha1.PodAvailableBadge) (*CueParams, error) {
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

func getObject(c dynamiclister.DynamicResourceLister, obj *unstructured.Unstructured, rs *policyv1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	gvk := schema.FromAPIVersionAndKind(rs.APIVersion, rs.Kind)

	lister, err := c.GVKToResourceLister(gvk)
	if err != nil {
		klog.ErrorS(err, "GetGroupVersionResource got error",
			"apiVersion", rs.APIVersion, "kind", rs.Kind, "name", rs.Name)
		return nil, err
	}

	if rs.Name != "" {
		refName, ok, err := parseAndGetRefValue(rs.Name, obj)
		if err != nil {
			return nil, err
		}
		if !ok {
			// ref not found return an empty obj
			return new(unstructured.Unstructured), nil
		}
		refNs, ok, err := parseAndGetRefValue(rs.Namespace, obj)
		if err != nil {
			return nil, err
		}
		if !ok {
			// ref not found return an empty obj
			return new(unstructured.Unstructured), nil
		}

		klog.V(4).InfoS("get owner reference", "apiVersion", rs.APIVersion, "kind", rs.Kind, "name", refName)
		obj, err := convertLister(lister, refNs).Get(refName)
		if err != nil {
			return nil, err
		}

		return obj.(*unstructured.Unstructured), nil
	}

	if rs.LabelSelector != nil {
		handled, ok, err := handleRefSelectLabels(rs.LabelSelector, obj)
		if err != nil {
			return nil, err
		}
		if !ok {
			// ref not found
			return new(unstructured.Unstructured), nil
		}
		// use selector
		s, err := metav1.LabelSelectorAsSelector(handled)
		if err != nil {
			return nil, err
		}
		// ns
		refNs, ok, err := parseAndGetRefValue(rs.Namespace, obj)
		if err != nil {
			return nil, err
		}
		if !ok {
			// ref not found return an empty obj
			return new(unstructured.Unstructured), nil
		}

		klog.V(4).InfoS("get object", "label", s.String())
		list, err := convertLister(lister, refNs).List(s)
		if err != nil {
			return nil, err
		}

		klog.V(4).InfoS("list by label", "list", list)
		if len(list) > 0 {
			return list[0].(*unstructured.Unstructured), nil
		}
	}

	return nil, nil
}

func handleRefSelectLabels(ls *metav1.LabelSelector, obj *unstructured.Unstructured) (*metav1.LabelSelector, bool, error) {
	result := &metav1.LabelSelector{
		MatchLabels:      make(map[string]string),
		MatchExpressions: make([]metav1.LabelSelectorRequirement, len(ls.MatchExpressions)),
	}
	for k, v := range ls.MatchLabels {
		refVal, ok, err := parseAndGetRefValue(v, obj)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			// ref not found
			return nil, false, nil
		}

		result.MatchLabels[k] = refVal
	}

	for i, expression := range ls.MatchExpressions {
		var values []string
		for _, value := range expression.Values {
			refVal, ok, err := parseAndGetRefValue(value, obj)
			if err != nil {
				return nil, false, err
			}
			if !ok {
				// ref not found
				return nil, false, nil
			}

			values = append(values, refVal)
		}

		result.MatchExpressions[i] = metav1.LabelSelectorRequirement{
			Key:      expression.Key,
			Operator: expression.Operator,
			Values:   values,
		}
	}

	return result, true, nil
}

func getOwnerReference(c dynamiclister.DynamicResourceLister, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	list := obj.GetOwnerReferences()
	if len(list) == 0 {
		return nil, errors.New("object has no owner reference")
	}

	or := list[0]
	gvk := schema.FromAPIVersionAndKind(or.APIVersion, or.Kind)
	klog.V(4).InfoS("get owner reference", "apiVersion", or.APIVersion, "kind", or.Kind, "name", or.Name)

	lister, err := c.GVKToResourceLister(gvk)
	if err != nil {
		klog.ErrorS(err, "GetGroupVersionResource got error", "apiVersion", or.APIVersion, "kind", or.Kind, "name", or.Name)
		return nil, err
	}

	if or.Name == "" {
		// return empty obj
		return new(unstructured.Unstructured), nil
	}

	result, err := convertLister(lister, obj.GetNamespace()).Get(or.Name)
	if err != nil {
		return nil, err
	}

	return result.(*unstructured.Unstructured), nil
}

func convertLister(l cache.GenericLister, ns string) cache.GenericNamespaceLister {
	if ns != "" {
		return l.ByNamespace(ns)
	}

	return l.(cache.GenericNamespaceLister)
}

// See shouldCopyHeaderOnRedirect https://golang.org/src/net/http/client.go
var secHeaders = []string{"Authorization", "Www-Authenticate", "Cookie", "Cookie2"}

var ErrTooManyRedirects = errors.New("too many redirects")

// support direct
var defaultHTTPClient = &http.Client{
	Timeout: time.Second,
	// Workaround security behavior in client where it may
	// discard certain security-related header on redirect.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 10 {
			// Emulate default redirect check.
			return ErrTooManyRedirects
		}
		if len(via) > 0 {
			for _, header := range secHeaders {
				if req.Header.Get(header) == "" {
					req.Header.Set(header, via[len(via)-1].Header.Get(header))
				}
			}
		}

		return nil
	},
}

func getHttpResponse(c *http.Client, obj *unstructured.Unstructured, ref *policyv1alpha1.HttpDataRef) (map[string]any, error) {
	if c == nil {
		c = defaultHTTPClient
	}

	var (
		params  string
		query   = url.Values{}
		reqBody io.Reader
	)
	for k, v := range ref.Params {
		refVal, ok, err := parseAndGetRefValue(v, obj)
		if err != nil {
			return nil, err
		}
		if !ok {
			// ref not found
			return map[string]any{}, nil
		}
		query.Set(k, refVal)
	}

	if len(ref.Body.Raw) > 0 {
		reqBody = bytes.NewBuffer(ref.Body.Raw)
	}

	if len(ref.Params) > 0 {
		params = "?" + query.Encode()
	}

	refUrl, ok, err := parseAndGetRefValue(ref.URL, obj)
	if err != nil {
		return nil, err
	}
	if !ok {
		// ref not found
		return map[string]any{}, nil
	}
	req, err := http.NewRequest(strings.ToUpper(ref.Method), refUrl+params, reqBody)
	if err != nil {
		return nil, err
	}
	for k, v := range ref.Header {
		req.Header.Set(k, v)
	}

	// check if request need auth
	if ref.Auth != nil {
		token, err := httpAuth(ref.Auth)
		if err != nil {
			klog.ErrorS(err, "auth http failed", "url", refUrl, "method", ref.Method)
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)
	}

	klog.V(4).InfoS("requesting http api", "url", refUrl, "method", ref.Method)
	resp, err := c.Do(req)
	if err != nil {
		klog.ErrorS(err, "request http api failed", "url", refUrl, "method", ref.Method)
		return nil, err
	}
	defer noErr(resp.Body.Close)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		klog.ErrorS(err, "read http body failed", "url", refUrl, "method", ref.Method)
		return nil, err
	}
	klog.V(4).InfoS("http response", "url", refUrl, "method", ref.Method, "body", string(b))

	respBody := make(map[string]any)
	if err = json.Unmarshal(b, &respBody); err != nil {
		klog.ErrorS(err, "unmarshal response body to map[string]any failed")
		return nil, err
	}

	return map[string]any{
		"body":    respBody,
		"header":  resp.Header,
		"trailer": resp.Trailer,
	}, nil
}

type Auth struct {
	Token string `json:"token"`
}

func httpAuth(a *policyv1alpha1.HttpRequestAuth) (token string, err error) {
	if a == nil {
		return "", errors.New("invalid auth")
	}

	if a.StaticToken != "" {
		return a.StaticToken, nil
	}

	// maintain by token manager
	if a.Token != "" {
		return a.Token, nil
	}

	return "", errors.New("no available token")
}

func noErr(f func() error) {
	_ = f()
}

var (
	matchRef = regexp.MustCompilePOSIX(`\{\{(.*?)\}\}`)
)

func matchRefValue(s string) string {
	result := matchRef.FindStringSubmatch(s)
	if len(result) > 1 {
		return result[1]
	}

	return ""
}

func parseAndGetRefValue(refKey string, obj *unstructured.Unstructured) (string, bool, error) {
	if strings.Contains(refKey, "{{}}") {
		return "", false, errors.New("invalid ref key")
	}

	key := matchRefValue(refKey)
	if key == "" {
		return refKey, true, nil
	}

	v, ok, err := unstructured.NestedString(obj.Object, strings.Split(key, ".")...)
	if err != nil {
		klog.ErrorS(err, "get reference value from current object got error", "key", key, "object", klog.KObj(obj))
		return "", false, fmt.Errorf("get reference value from current object got error,err=%w", err)
	}

	// ref not found
	if !ok {
		klog.ErrorS(err, "get reference value from current object is not string", "key", key, "object", klog.KObj(obj))
		return "", false, nil
	}

	return strings.Replace(refKey, fmt.Sprintf("{{%s}}", key), v, 1), true, nil
}
