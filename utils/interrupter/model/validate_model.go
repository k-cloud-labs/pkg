package model

import (
	"bytes"
	"encoding/json"

	"github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type ValidatePolicyRenderData struct {
	Cond      string
	Value     *v1alpha1.CustomTypes
	ValueType v1alpha1.ValueType
	ValueRef  *ResourceRefer
	DataRef   *ResourceRefer
	Message   string
}

func (vrd *ValidatePolicyRenderData) String() string {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "\t")

	if err := enc.Encode(vrd); err != nil {
		return ""
	}

	return buf.String()
}

type ResourceRefer struct {
	From v1alpha1.ValueRefFrom
	// will convert to cue reference
	CueObjectKey string
	Path         string
}

func ValidateRulesToValidatePolicyRenderData(vc *v1alpha1.TemplateCondition) *ValidatePolicyRenderData {
	nvc := &ValidatePolicyRenderData{
		Cond:     convertCond(vc.Cond),
		Value:    vc.Value,
		ValueRef: convertResourceRefer("", vc.ValueRef),
		DataRef:  convertResourceRefer("_d", vc.DataRef),
		Message:  vc.Message,
	}

	if nvc.Value != nil {
		nvc.ValueType = v1alpha1.ValueTypeConst
	}

	if nvc.ValueRef != nil {
		nvc.ValueType = v1alpha1.ValueTypeRefer
	}

	return nvc
}

func convertResourceRefer(suffix string, rf *v1alpha1.ResourceRefer) *ResourceRefer {
	if rf == nil {
		return nil
	}
	nrf := &ResourceRefer{
		From: rf.From,
		Path: handlePath(rf.Path),
	}

	switch rf.From {
	case v1alpha1.FromCurrentObject:
		nrf.CueObjectKey = "object"
	case v1alpha1.FromOldObject:
		nrf.CueObjectKey = "oldObject"
	case v1alpha1.FromK8s, v1alpha1.FromOwnerReference:
		nrf.CueObjectKey = "otherObject" + suffix
	case v1alpha1.FromHTTP:
		nrf.CueObjectKey = "http" + suffix
	}

	return nrf
}

func convertCond(c v1alpha1.Cond) string {
	switch c {
	case v1alpha1.CondEqual:
		return "=="
	case v1alpha1.CondNotEqual:
		return "!="
	case v1alpha1.CondGreaterOrEqual:
		return ">="
	case v1alpha1.CondGreater:
		return ">"
	case v1alpha1.CondLesserOrEqual:
		return "<="
	case v1alpha1.CondLesser:
		return "<"
	default:
		return string(c)
	}
}
