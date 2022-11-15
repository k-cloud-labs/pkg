package templatemanager

import (
	"bytes"
	"encoding/json"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/interrupter/model"
)

// NewOverrideTemplateManager init override policy template manager.
func NewOverrideTemplateManager(ts *TemplateSource) (TemplateManager, error) {
	t, err := NewTemplateManager(ts,
		template.FuncMap{
			"marshal": func(v interface{}) string {
				buf := &bytes.Buffer{}
				en := json.NewEncoder(buf)
				if err := en.Encode(v); err != nil {
					return ""
				}

				return buf.String()
			},
			"isValidOp": func(v interface{}) bool {
				rule, ok := v.(*model.OverridePolicyRenderData)
				if !ok {
					return false
				}
				return validateOp(rule)
			},
			"convertQuantity": func(v interface{}) int64 {
				q, ok := v.(resource.Quantity)
				if !ok {
					return 0
				}

				return q.Value()
			},
			// only support string map
			"isMap": func(v interface{}) bool {
				_, ok := v.(map[string]string)
				return ok
			},
			// convert to map
			"convertToMap": func(v interface{}) map[string]string {
				m, ok := v.(map[string]string)
				if ok {
					return m
				}

				return map[string]string{}
			},
		})
	if err != nil {
		return nil, err
	}

	return t, nil
}

func validateOp(rule *model.OverridePolicyRenderData) bool {
	switch rule.Type {
	case policyv1alpha1.OverrideRuleTypeAnnotations:
		return true
	case policyv1alpha1.OverrideRuleTypeLabels:
		return true
	case policyv1alpha1.OverrideRuleTypeResourcesOversell:
		return rule.ResourcesOversell != nil
	case policyv1alpha1.OverrideRuleTypeResources:
		return rule.Resources != nil
	case policyv1alpha1.OverrideRuleTypeTolerations:
		return true
	case policyv1alpha1.OverrideRuleTypeAffinity:
		return validateAffinity(rule)
	}

	return false
}

func validateAffinity(rule *model.OverridePolicyRenderData) bool {
	a := rule.Affinity
	if a == nil {
		return false
	}

	if a.NodeAffinity == nil && a.PodAffinity == nil && a.PodAntiAffinity == nil {
		return false
	}

	return isNodeAffinityValid(a.NodeAffinity) || isPodAffinityValid(a.PodAffinity) || isPodAntiAffinityValid(a.PodAntiAffinity)
}

func isNodeAffinityValid(na *corev1.NodeAffinity) bool {
	if na == nil {
		return false
	}

	isRequireValid := func(req *corev1.NodeSelector) bool {
		if req == nil {
			return false
		}

		if len(na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0 {
			return false
		}

		for _, term := range na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			if len(term.MatchExpressions) > 0 || len(term.MatchFields) > 0 {
				return true
			}
		}

		return false
	}
	isPreferredValid := func(list []corev1.PreferredSchedulingTerm) bool {
		if len(list) == 0 {
			return false
		}

		for _, term := range list {
			if len(term.Preference.MatchExpressions) > 0 || len(term.Preference.MatchFields) > 0 {
				return true
			}
		}

		return false
	}

	return isRequireValid(na.RequiredDuringSchedulingIgnoredDuringExecution) ||
		isPreferredValid(na.PreferredDuringSchedulingIgnoredDuringExecution)
}

func isPodAffinityValid(pa *corev1.PodAffinity) bool {
	if pa == nil {
		return false
	}

	return len(pa.RequiredDuringSchedulingIgnoredDuringExecution) > 0 ||
		len(pa.PreferredDuringSchedulingIgnoredDuringExecution) > 0
}

func isPodAntiAffinityValid(pa *corev1.PodAntiAffinity) bool {
	if pa == nil {
		return false
	}

	return len(pa.RequiredDuringSchedulingIgnoredDuringExecution) > 0 ||
		len(pa.PreferredDuringSchedulingIgnoredDuringExecution) > 0
}
