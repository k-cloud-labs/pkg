package cue

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type CueParams struct {
	Object    *unstructured.Unstructured
	OldObject *unstructured.Unstructured
	// k8s_object:xxx, http:xxx
	ExtraParams map[string]any
}

func BuildCueParamsViaOverridePolicy(c dynamic.Interface, overriders *policyv1alpha1.Overriders) (*CueParams, error) {
	var (
		cp = &CueParams{
			ExtraParams: make(map[string]any),
		}
		rule = overriders.Template
	)
	if rule.ValueRef != nil {
		if rule.ValueRef.From == policyv1alpha1.FromK8s {
			obj, err := getObject(c, rule.ValueRef.K8s)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["otherObject"] = obj
		}

		if rule.ValueRef.From == policyv1alpha1.FromHTTP {
			obj, err := getHttpResponse(nil, rule.ValueRef.Http)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["http"] = obj
		}
	}

	return cp, nil
}

func BuildCueParamsViaValidatePolicy(c dynamic.Interface, condition *policyv1alpha1.TemplateCondition) (*CueParams, error) {
	var cp = &CueParams{
		ExtraParams: make(map[string]any),
	}

	if condition.ValueRef != nil {
		if condition.ValueRef.From == policyv1alpha1.FromK8s {
			obj, err := getObject(c, condition.ValueRef.K8s)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["otherObject"] = obj
		}

		if condition.ValueRef.From == policyv1alpha1.FromHTTP {
			obj, err := getHttpResponse(nil, condition.ValueRef.Http)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["http"] = obj
		}
	}

	if condition.DataRef != nil {
		if condition.DataRef.From == policyv1alpha1.FromK8s {
			obj, err := getObject(c, condition.DataRef.K8s)
			if err != nil {
				return nil, err
			}
			// _d for dataRef
			cp.ExtraParams["otherObject_d"] = obj
		}

		if condition.DataRef.From == policyv1alpha1.FromHTTP {
			obj, err := getHttpResponse(nil, condition.DataRef.Http)
			if err != nil {
				return nil, err
			}
			cp.ExtraParams["http_d"] = obj
		}
	}

	return cp, nil
}

func getObject(c dynamic.Interface, rs *policyv1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	gvk := schema.GroupVersionResource{
		Version:  rs.APIVersion,
		Resource: rs.Kind,
	}

	if rs.Name != "" {
		obj, err := c.Resource(gvk).Namespace(rs.Namespace).Get(ctx, rs.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return obj, nil
	}

	if rs.LabelSelector != nil {
		// use selector
		s, err := metav1.LabelSelectorAsSelector(rs.LabelSelector)
		if err != nil {
			return nil, err
		}

		list, err := c.Resource(gvk).Namespace(rs.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: s.String(),
		})
		if err != nil {
			return nil, err
		}

		if len(list.Items) > 0 {
			return &list.Items[0], nil
		}
	}

	return nil, nil
}

func getHttpResponse(c *http.Client, ref *policyv1alpha1.HttpDataRef) (map[string]any, error) {
	if c == nil {
		c = &http.Client{
			Transport: http.DefaultTransport,
			Timeout:   time.Second * 3,
		}
	}

	val := url.Values{}
	for k, v := range ref.Params {
		val.Set(k, v)
	}
	req, err := http.NewRequest(ref.Method, ref.URL+"?"+val.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
