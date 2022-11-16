package interrupter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/interrupter/model"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
)

// PolicyInterrupter defines interrupt process for policy change
// It validate and mutate policy.
type PolicyInterrupter interface {
	// OnValidating called on "/validating" api to validate policy
	// return nil means obj is not defined policy or no invalid field
	OnValidating(obj, oldObj *unstructured.Unstructured) error
	// OnMutating called on "/mutating" api to complete policy
	// return nil means obj is not defined policy
	OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error)
}

type policyInterrupterImpl struct {
	overrideTemplateManager templatemanager.TemplateManager
	validateTemplateManager templatemanager.TemplateManager
	cueManager              templatemanager.CueManager
}

const (
	stageValidate = "validate"
	stageMutate   = "mutate"
)

func (p *policyInterrupterImpl) OnValidating(obj, oldObj *unstructured.Unstructured) error {
	_, err := p.interrupt(obj, oldObj, stageValidate)
	return err
}

func (p *policyInterrupterImpl) OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error) {
	return p.interrupt(obj, oldObj, stageMutate)
}

func (p *policyInterrupterImpl) interrupt(obj, oldObj *unstructured.Unstructured, stage string) (
	patches []jsonpatchv2.JsonPatchOperation, err error) {
	group := strings.Split(obj.GetAPIVersion(), "/")[0]
	if group != policyv1alpha1.SchemeGroupVersion.Group {
		return
	}

	klog.Infof("policy changed. apiVersion=%v, kind=%v, stage=%v", obj.GetAPIVersion(), obj.GetKind(), stage)

	// check crd type before call this func
	kind := obj.GetKind()
	switch kind {
	case "ClusterValidatePolicy":
		patches, err = p.onClusterValidationPolicyChange(obj, oldObj, stage)
	case "ClusterOverridePolicy":
		patches, err = p.onClusterOverridePolicyChange(obj, oldObj, stage)
	case "OverridePolicy":
		patches, err = p.onOverridePolicyChange(obj, oldObj, stage)
	default:
		return
	}

	if err != nil {
		klog.ErrorS(err, "interrupt policy error",
			"kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
		return nil, err
	}

	klog.V(4).Infof("policy interrupt result.rsp=%+v", patches)
	if stage == stageMutate {
		if err := applyJSONPatch(obj, patches); err != nil {
			klog.ErrorS(err, "apply json patch error",
				"kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
			return nil, err
		}
	}

	return
}

func (p *policyInterrupterImpl) onClusterValidationPolicyChange(obj, oldObj *unstructured.Unstructured, stage string) ([]jsonpatchv2.JsonPatchOperation, error) {
	cvp := new(policyv1alpha1.ClusterValidatePolicy)
	if err := convertToPolicy(obj, cvp); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.ClusterValidatePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterValidatePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return nil, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(cvp.Spec, old.Spec) {
			return nil, nil
		}
	}

	switch stage {
	case stageValidate:
		return nil, p.validateClusterValidatePolicy(cvp)
	case stageMutate:
		return p.patchClusterValidatePolicy(cvp)
	default:
		return nil, errors.New("unknown stage")
	}
}

func (p *policyInterrupterImpl) onOverridePolicyChange(obj, oldObj *unstructured.Unstructured, stage string) ([]jsonpatchv2.JsonPatchOperation, error) {
	op := new(policyv1alpha1.OverridePolicy)
	if err := convertToPolicy(obj, op); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.OverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.OverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return nil, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(op.Spec, old.Spec) {
			return nil, nil
		}
	}

	switch stage {
	case stageValidate:
		return nil, p.validateOverridePolicy(&op.Spec)
	case stageMutate:
		return p.patchOverridePolicy(&op.Spec)
	default:
		return nil, errors.New("unknown stage")

	}
}

func (p *policyInterrupterImpl) onClusterOverridePolicyChange(obj, oldObj *unstructured.Unstructured, stage string) ([]jsonpatchv2.JsonPatchOperation, error) {
	cop := new(policyv1alpha1.ClusterOverridePolicy)
	if err := convertToPolicy(obj, cop); err != nil {
		return nil, err
	}

	var old *policyv1alpha1.ClusterOverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterOverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return nil, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(cop.Spec, old.Spec) {
			return nil, nil
		}
	}

	switch stage {
	case stageValidate:
		return nil, p.validateOverridePolicy(&cop.Spec)
	case stageMutate:
		return p.patchOverridePolicy(&cop.Spec)
	default:
		return nil, errors.New("unknown stage")

	}
}

func (p *policyInterrupterImpl) patchClusterValidatePolicy(obj *policyv1alpha1.ClusterValidatePolicy) ([]jsonpatchv2.JsonPatchOperation, error) {
	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	for i, validateRule := range obj.Spec.ValidateRules {
		if validateRule.Template == nil {
			continue
		}

		if validateRule.Template.Condition != nil && validateRule.Template.Condition.AffectMode == "" {
			validateRule.Template.Condition.AffectMode = policyv1alpha1.AffectModeReject
			patches = append(patches, jsonpatchv2.JsonPatchOperation{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/validateRules/%d/template/condition/affectMode", i),
				Value:     policyv1alpha1.AffectModeReject,
			})
		}
		if validateRule.Template.PodAvailableBadge != nil &&
			validateRule.Template.PodAvailableBadge.ReplicaReference == nil {
			validateRule.Template.PodAvailableBadge.ReplicaReference = &policyv1alpha1.ReplicaResourceRefer{
				From:               policyv1alpha1.FromOwnerReference,
				TargetReplicaPath:  "/spec/replica",
				CurrentReplicaPath: "/status/replica",
			}

			b, err := json.Marshal(validateRule.Template.PodAvailableBadge.ReplicaReference)
			if err != nil {
				return nil, err
			}
			patches = append(patches, jsonpatchv2.JsonPatchOperation{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/validateRules/%d/template/podAvailableBadge/replicaReference", i),
				Value:     string(b),
			})
		}

		b, err := p.renderAndFormat(validateRule.Template)
		if err != nil {
			return nil, err
		}

		patches = append(patches, jsonpatchv2.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/validateRules/%d/renderedCue", i),
			Value:     string(b),
		})
	}

	return patches, nil
}

func (p *policyInterrupterImpl) patchOverridePolicy(objSpec *policyv1alpha1.OverridePolicySpec) ([]jsonpatchv2.JsonPatchOperation, error) {
	patches := make([]jsonpatchv2.JsonPatchOperation, 0)
	for i, overrideRule := range objSpec.OverrideRules {
		if overrideRule.Overriders.Template == nil {
			continue
		}

		b, err := p.renderAndFormat(overrideRule.Overriders.Template)
		if err != nil {
			return nil, err
		}

		patches = append(patches, jsonpatchv2.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/overrideRules/%d/overriders/renderedCue", i),
			Value:     string(b),
		})
	}

	return patches, nil
}

func (p *policyInterrupterImpl) renderAndFormat(data any) (b []byte, err error) {
	switch tmpl := data.(type) {
	case *policyv1alpha1.OverrideRuleTemplate:
		mrd := model.OverrideRulesToOverridePolicyRenderData(tmpl)
		b, err := p.overrideTemplateManager.Render(mrd)
		if err != nil {
			return nil, err
		}

		return p.cueManager.Format(trimBlankLine(b))
	case *policyv1alpha1.ValidateRuleTemplate:
		vrd := model.ValidateRulesToValidatePolicyRenderData(tmpl)
		b, err := p.validateTemplateManager.Render(vrd)
		if err != nil {
			return nil, err
		}

		return p.cueManager.Format(trimBlankLine(b))
	}

	return nil, errors.New("unknown data type")
}

func (p *policyInterrupterImpl) validateClusterValidatePolicy(obj *policyv1alpha1.ClusterValidatePolicy) error {
	for _, validateRule := range obj.Spec.ValidateRules {
		if validateRule.Template == nil || len(validateRule.RenderedCue) == 0 {
			continue
		}
		if err := p.cueManager.Validate([]byte(validateRule.RenderedCue)); err != nil {
			return err
		}
	}

	return nil
}

func (p *policyInterrupterImpl) validateOverridePolicy(objSpec *policyv1alpha1.OverridePolicySpec) error {
	for _, overrideRule := range objSpec.OverrideRules {
		if err := p.cueManager.Validate([]byte(overrideRule.Overriders.RenderedCue)); err != nil {
			return err
		}
	}

	return nil
}

// applyJSONPatch applies the override on to the given unstructured object.
func applyJSONPatch(obj *unstructured.Unstructured, overrides []jsonpatchv2.JsonPatchOperation) error {
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return err
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return err
	}

	objectJSONBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}

	patchedObjectJSONBytes, err := patch.Apply(objectJSONBytes)
	if err != nil {
		return err
	}

	err = obj.UnmarshalJSON(patchedObjectJSONBytes)
	return err
}

func convertToPolicy(u *unstructured.Unstructured, data any) error {
	klog.V(4).Infof("convertToPolicy, obj=%v", u)
	b, err := u.MarshalJSON()
	if err != nil {
		return err
	}

	return json.Unmarshal(b, data)
}

func trimBlankLine(data []byte) []byte {
	s := bufio.NewScanner(bytes.NewBuffer(data))
	var result = &bytes.Buffer{}
	for s.Scan() {
		line := s.Text()
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		result.Write(s.Bytes())
		result.WriteByte('\n')
	}

	return result.Bytes()
}

func NewPolicyInterrupter(mtm, vtm templatemanager.TemplateManager, cm templatemanager.CueManager) PolicyInterrupter {
	return &policyInterrupterImpl{
		overrideTemplateManager: mtm,
		validateTemplateManager: vtm,
		cueManager:              cm,
	}
}
