package templatemanager

import (
	"bytes"
	"encoding/json"
	"text/template"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/interrupter/model"
)

// NewOverrideTemplateManager init override policy template manager.
func NewOverrideTemplateManager(filePath string) (TemplateManager, error) {
	t, err := NewTemplateManager(
		&TemplateSource{
			FilePath:     filePath,
			TemplateName: "BaseTemplate",
		},
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
	case v1alpha1.RuleTypeAnnotations:
		return true
	case v1alpha1.RuleTypeLabels:
		return true
	case v1alpha1.RuleTypeResourcesOversell:
		return rule.ResourcesOversell != nil
	case v1alpha1.RuleTypeResources:
		return rule.Resources != nil
	case v1alpha1.RuleTypeTolerations:
		return true
	case v1alpha1.RuleTypeAffinity:
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

func isNodeAffinityValid(na *v1.NodeAffinity) bool {
	if na == nil {
		return false
	}

	isRequireValid := func(req *v1.NodeSelector) bool {
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
	isPreferredValid := func(list []v1.PreferredSchedulingTerm) bool {
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

func isPodAffinityValid(pa *v1.PodAffinity) bool {
	if pa == nil {
		return false
	}

	return len(pa.RequiredDuringSchedulingIgnoredDuringExecution) > 0 ||
		len(pa.PreferredDuringSchedulingIgnoredDuringExecution) > 0
}

func isPodAntiAffinityValid(pa *v1.PodAntiAffinity) bool {
	if pa == nil {
		return false
	}

	return len(pa.RequiredDuringSchedulingIgnoredDuringExecution) > 0 ||
		len(pa.PreferredDuringSchedulingIgnoredDuringExecution) > 0
}
