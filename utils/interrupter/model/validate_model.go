package model

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

type ValidatePolicyRenderData struct {
	Type      string
	Condition *ValidateCondition
	PAB       *PodAvailableBadge
}

type ValidateCondition struct {
	Cond         string
	Value        *policyv1alpha1.ConstantValue
	ValueType    policyv1alpha1.ValueType
	ValueRef     *ResourceRefer
	DataRef      *ResourceRefer
	ValueProcess *ValueProcess
	Message      string
}

type PodAvailableBadge struct {
	IsPercentage     bool
	MaxUnavailable   *float64
	MinAvailable     *float64
	ReplicaReference *ReplicaResourceRefer
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
	From policyv1alpha1.ValueRefFrom
	// will convert to cue reference
	CueObjectKey string
	Path         string
}

type ReplicaResourceRefer struct {
	From policyv1alpha1.ValueRefFrom
	// will convert to cue reference
	CueObjectKey       string
	TargetReplicaPath  string
	CurrentReplicaPath string
}

type ValueProcess struct {
	Operation     policyv1alpha1.OperationType
	OperationWith float64
}

func ValidateRulesToValidatePolicyRenderData(vc *policyv1alpha1.ValidateRuleTemplate) *ValidatePolicyRenderData {
	return &ValidatePolicyRenderData{
		Type:      string(vc.Type),
		Condition: convertGeneralCondition(vc.Condition),
		PAB:       convertPAB(vc.PodAvailableBadge),
	}
}

func convertGeneralCondition(vc *policyv1alpha1.ValidateCondition) *ValidateCondition {
	if vc == nil {
		return nil
	}

	nvc := &ValidateCondition{
		Cond:     convertCond(vc.Cond),
		Value:    vc.Value,
		ValueRef: convertResourceRefer("", vc.ValueRef),
		DataRef:  convertResourceRefer("_d", vc.DataRef),
		Message:  vc.Message,
	}

	if nvc.Value != nil {
		nvc.ValueType = policyv1alpha1.ValueTypeConst
	}

	if nvc.ValueRef != nil {
		nvc.ValueType = policyv1alpha1.ValueTypeRefer
	}

	if vc.ValueProcess != nil {
		nvc.ValueProcess = &ValueProcess{
			Operation: vc.ValueProcess.Operation,
		}

		if vc.ValueProcess.OperationWith.Type == intstr.Int {
			nvc.ValueProcess.OperationWith = float64(vc.ValueProcess.OperationWith.IntVal)
			return nvc
		}

		var (
			str          = vc.ValueProcess.OperationWith.StrVal
			isPercentage = strings.HasSuffix(str, "%")
		)
		// is percentage
		if isPercentage {
			str = str[:len(str)-1]
		}

		// ignore error since validate the value when create crd
		f, _ := strconv.ParseFloat(str, 64)
		if isPercentage {
			f /= 100
		}

		nvc.ValueProcess.OperationWith = f
	}

	return nvc
}

func convertPAB(pp *policyv1alpha1.PodAvailableBadge) *PodAvailableBadge {
	if pp == nil {
		return nil
	}
	var (
		mu           *float64
		ma           *float64
		isPercentage bool
	)
	mu, isPercentage = intStr2FloatPtr(pp.MaxUnavailable)
	if mu == nil {
		ma, isPercentage = intStr2FloatPtr(pp.MinAvailable)
	}

	return &PodAvailableBadge{
		IsPercentage:     isPercentage,
		MaxUnavailable:   mu,
		MinAvailable:     ma,
		ReplicaReference: convertReplicaResourceRefer("", pp.ReplicaReference),
	}
}

func intStr2FloatPtr(is *intstr.IntOrString) (*float64, bool) {
	if is == nil {
		return nil, false
	}
	var f float64
	if is.Type == intstr.Int {
		f = float64(is.IntValue())
		return &f, false
	}

	// isPercentage
	str := is.StrVal
	isPercentage := strings.HasSuffix(is.StrVal, "%")
	if isPercentage {
		str = str[:len(str)-1]
	}

	// check str when create
	f, _ = strconv.ParseFloat(str, 64)
	if isPercentage && f != 0 {
		f /= 100
	}

	return &f, isPercentage

}

func convertReplicaResourceRefer(suffix string, rf *policyv1alpha1.ReplicaResourceRefer) *ReplicaResourceRefer {
	if rf == nil {
		return nil
	}
	nrf := &ReplicaResourceRefer{
		From:               rf.From,
		TargetReplicaPath:  handlePath(rf.TargetReplicaPath),
		CurrentReplicaPath: handlePath(rf.CurrentReplicaPath),
	}

	switch rf.From {
	case policyv1alpha1.FromCurrentObject:
		nrf.CueObjectKey = "object"
	case policyv1alpha1.FromOldObject:
		nrf.CueObjectKey = "oldObject"
	case policyv1alpha1.FromK8s, policyv1alpha1.FromOwnerReference:
		nrf.CueObjectKey = "otherObject" + suffix
	case policyv1alpha1.FromHTTP:
		nrf.CueObjectKey = "http" + suffix
	}

	return nrf
}

func convertResourceRefer(suffix string, rf *policyv1alpha1.ResourceRefer) *ResourceRefer {
	if rf == nil {
		return nil
	}
	nrf := &ResourceRefer{
		From: rf.From,
		Path: handlePath(rf.Path),
	}

	switch rf.From {
	case policyv1alpha1.FromCurrentObject:
		nrf.CueObjectKey = "object"
	case policyv1alpha1.FromOldObject:
		nrf.CueObjectKey = "oldObject"
	case policyv1alpha1.FromK8s, policyv1alpha1.FromOwnerReference:
		nrf.CueObjectKey = "otherObject" + suffix
	case policyv1alpha1.FromHTTP:
		nrf.CueObjectKey = "http" + suffix
	}

	return nrf
}

func convertCond(c policyv1alpha1.Cond) string {
	switch c {
	case policyv1alpha1.CondEqual:
		return "=="
	case policyv1alpha1.CondNotEqual:
		return "!="
	case policyv1alpha1.CondGreaterOrEqual:
		return ">="
	case policyv1alpha1.CondGreater:
		return ">"
	case policyv1alpha1.CondLesserOrEqual:
		return "<="
	case policyv1alpha1.CondLesser:
		return "<"
	default:
		return string(c)
	}
}
