package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type OverridePolicyRenderData struct {
	Type      policyv1alpha1.OverrideRuleType
	Op        policyv1alpha1.OverriderOperator
	Path      string
	Value     any
	ValueType policyv1alpha1.ValueType
	ValueRef  *ResourceRefer

	//resource
	Resources *corev1.ResourceRequirements
	// resource oversell
	ResourcesOversell *policyv1alpha1.ResourcesOversellRule
	// toleration
	Tolerations []*corev1.Toleration
	// affinity
	Affinity *corev1.Affinity
}

func (mrd *OverridePolicyRenderData) String() string {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "\t")

	if err := enc.Encode(mrd); err != nil {
		return ""
	}

	return buf.String()
}

func OverrideRulesToOverridePolicyRenderData(or *policyv1alpha1.OverrideRuleTemplate) *OverridePolicyRenderData {
	nr := &OverridePolicyRenderData{
		Type:              or.Type,
		Op:                or.Operation,
		Value:             or.Value.Value(),
		Path:              handlePath(or.Path),
		Resources:         or.Resources,
		ResourcesOversell: or.ResourcesOversell,
		Tolerations:       or.Tolerations,
		Affinity:          or.Affinity,
	}
	switch or.Type {
	case policyv1alpha1.OverrideRuleTypeAnnotations:
		fallthrough
	case policyv1alpha1.OverrideRuleTypeLabels:
		nr.ValueType = policyv1alpha1.ValueTypeRefer
		if or.Value != nil {
			nr.ValueType = policyv1alpha1.ValueTypeConst
			break
		}

		if or.ValueRef != nil {
			vr := &ResourceRefer{
				From: or.ValueRef.From,
				Path: handlePath(or.ValueRef.Path),
			}
			switch or.ValueRef.From {
			case policyv1alpha1.FromCurrentObject:
				vr.CueObjectKey = "object"
			case policyv1alpha1.FromOldObject:
				vr.CueObjectKey = "oldObject"
			case policyv1alpha1.FromK8s, policyv1alpha1.FromOwnerReference:
				vr.CueObjectKey = "otherObject"
			case policyv1alpha1.FromHTTP:
				vr.CueObjectKey = "http"
			}

			nr.ValueRef = vr
		}
	case policyv1alpha1.OverrideRuleTypeResourcesOversell:
		if or.ResourcesOversell != nil {
			if !or.ResourcesOversell.CpuFactor.ValidFactor() &&
				!or.ResourcesOversell.MemoryFactor.ValidFactor() &&
				!or.ResourcesOversell.DiskFactor.ValidFactor() {
				nr.ResourcesOversell = nil
			}

		}
	}

	return nr
}

var (
	numberRegex *regexp.Regexp
)

func init() {
	numberRegex = regexp.MustCompile(`^\d+$`)
}

func handlePath(path string) string {
	var slice []string
	// only handle /xxx/xxx/0/xxx pattern
	if !strings.Contains(path, "/") {
		return path
	}
	slice = strings.Split(strings.Trim(path, "/"), "/")

	result := make([]string, 0, len(slice))
	for i, s := range slice {
		if strings.Contains(s, "-") {
			s = fmt.Sprintf("\"%s\"", s)
		}

		if numberRegex.MatchString(s) {
			s = fmt.Sprintf("[%v]", s)
		} else if i > 0 {
			// non-index(numbers) elements add dot(.) in front, except first element
			result = append(result, ".")

		}

		result = append(result, s)

	}

	return strings.Join(result, "")
}
