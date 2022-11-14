package interrupter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/interrupter/model"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
)

// PolicyInterrupter defines interrupt process for policy change
type PolicyInterrupter interface {
	// OnValidating called when validate policy
	// return nil means obj is not defined policy
	OnValidating(obj, oldObj *unstructured.Unstructured) *admission.Response
	// OnMutating called when policy change
	// return nil means obj is not defined policy
	OnMutating(obj, oldObj *unstructured.Unstructured) *admission.Response
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

func (p *policyInterrupterImpl) OnValidating(obj, oldObj *unstructured.Unstructured) *admission.Response {
	return p.interrupt(obj, oldObj, stageValidate)
}

func (p *policyInterrupterImpl) OnMutating(obj, oldObj *unstructured.Unstructured) *admission.Response {
	return p.interrupt(obj, oldObj, stageMutate)
}

func (p *policyInterrupterImpl) interrupt(obj, oldObj *unstructured.Unstructured, stage string) *admission.Response {
	if obj.GetAPIVersion() != "policy.kcloudlabs.io/policyv1alpha1" {
		return nil
	}

	klog.Infof("policy changed. apiVersion=%v, kind=%v, stage=%v", obj.GetAPIVersion(), obj.GetKind(), stage)

	// check crd type before call this func
	kind := obj.GetKind()
	var (
		rsp admission.Response
		err error
	)
	switch kind {
	case "ClusterValidatePolicy":
		rsp, err = p.onClusterValidationPolicyChange(obj, oldObj, stage)
	case "ClusterOverridePolicy":
		rsp, err = p.onClusterOverridePolicyChange(obj, oldObj, stage)
	case "OverridePolicy":
		rsp, err = p.onOverridePolicyChange(obj, oldObj, stage)
	default:
		return nil
	}

	if err != nil {
		klog.ErrorS(err, "interrupt policy error",
			"kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
		rsp = admission.Errored(http.StatusInternalServerError, err)
		return &rsp
	}

	klog.V(4).Infof("policy interrupt result.rsp=%+v", rsp)
	if stage == stageMutate {
		if err := applyJSONPatch(obj, rsp.Patches); err != nil {
			klog.ErrorS(err, "apply json patch error",
				"kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
			rsp = admission.Errored(http.StatusInternalServerError, err)
			return &rsp
		}
	}

	return &rsp
}

func (p *policyInterrupterImpl) onClusterValidationPolicyChange(obj, oldObj *unstructured.Unstructured, stage string) (admission.Response, error) {
	cvp := new(policyv1alpha1.ClusterValidatePolicy)
	if err := convertToPolicy(obj, cvp); err != nil {
		return admission.Response{}, err
	}

	var old *policyv1alpha1.ClusterValidatePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterValidatePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return admission.Response{}, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(cvp.Spec, old.Spec) {
			return admission.Allowed("ok"), nil
		}
	}

	switch stage {
	case stageValidate:
		return p.validateClusterValidatePolicy(cvp)
	case stageMutate:
		return p.patchClusterValidatePolicy(cvp)
	default:
		return admission.Allowed("ok"), nil

	}
}

func (p *policyInterrupterImpl) onOverridePolicyChange(obj, oldObj *unstructured.Unstructured, stage string) (admission.Response, error) {
	op := new(policyv1alpha1.OverridePolicy)
	if err := convertToPolicy(obj, op); err != nil {
		return admission.Response{}, err
	}

	var old *policyv1alpha1.OverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.OverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return admission.Response{}, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(op.Spec, old.Spec) {
			return admission.Allowed("ok"), nil
		}
	}

	switch stage {
	case stageValidate:
		return p.validateOverridePolicy(&op.Spec)
	case stageMutate:
		return p.patchOverridePolicy(&op.Spec)
	default:
		return admission.Response{}, errors.New("unknown stage")

	}
}

func (p *policyInterrupterImpl) onClusterOverridePolicyChange(obj, oldObj *unstructured.Unstructured, stage string) (admission.Response, error) {
	cop := new(policyv1alpha1.ClusterOverridePolicy)
	if err := convertToPolicy(obj, cop); err != nil {
		return admission.Response{}, err
	}

	var old *policyv1alpha1.ClusterOverridePolicy
	if oldObj != nil {
		old = new(policyv1alpha1.ClusterOverridePolicy)
		if err := convertToPolicy(oldObj, old); err != nil {
			return admission.Response{}, err
		}
	}

	// UPDATE op
	if old != nil {
		// no change
		if reflect.DeepEqual(cop.Spec, old.Spec) {
			return admission.Allowed("ok"), nil
		}
	}

	switch stage {
	case stageValidate:
		return p.validateOverridePolicy(&cop.Spec)
	case stageMutate:
		return p.patchOverridePolicy(&cop.Spec)
	default:
		return admission.Response{}, errors.New("unknown stage")

	}
}

func (p *policyInterrupterImpl) patchClusterValidatePolicy(obj *policyv1alpha1.ClusterValidatePolicy) (admission.Response, error) {
	rsp := admission.Response{}
	for i, validateRule := range obj.Spec.ValidateRules {
		if validateRule.Template == nil {
			continue
		}

		if validateRule.Template.AffectMode == "" {
			validateRule.Template.AffectMode = policyv1alpha1.AffectModeReject
			rsp.Patches = append(rsp.Patches, jsonpatchv2.JsonPatchOperation{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/validateRules/%d/template/affectMode", i),
				Value:     policyv1alpha1.AffectModeReject,
			})
		}

		b, err := p.renderAndFormat(validateRule.Template)
		if err != nil {
			return admission.Response{}, err
		}

		rsp.Patches = append(rsp.Patches, jsonpatchv2.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/validateRules/%d/renderedCue", i),
			Value:     string(b),
		})
	}

	return rsp, nil
}

func (p *policyInterrupterImpl) patchOverridePolicy(objSpec *policyv1alpha1.OverridePolicySpec) (admission.Response, error) {
	rsp := admission.Response{}
	for i, overrideRule := range objSpec.OverrideRules {
		if overrideRule.Overriders.Template == nil {
			continue
		}

		b, err := p.renderAndFormat(overrideRule.Overriders.Template)
		if err != nil {
			return admission.Response{}, err
		}

		rsp.Patches = append(rsp.Patches, jsonpatchv2.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/overrideRules/%d/overriders/renderedCue", i),
			Value:     string(b),
		})
	}

	return rsp, nil
}

func (p *policyInterrupterImpl) renderAndFormat(data any) (b []byte, err error) {
	switch list := data.(type) {
	case *policyv1alpha1.TemplateRule:
		mrd := model.OverrideRulesToOverridePolicyRenderData(list)
		b, err := p.overrideTemplateManager.Render(mrd)
		if err != nil {
			return nil, err
		}

		return p.cueManager.Format(trimBlankLine(b))
	case *policyv1alpha1.TemplateCondition:
		vrd := model.ValidateRulesToValidatePolicyRenderData(list)
		b, err := p.validateTemplateManager.Render(vrd)
		if err != nil {
			return nil, err
		}

		return p.cueManager.Format(trimBlankLine(b))
	}

	return nil, errors.New("unknown data type")
}

func (p *policyInterrupterImpl) validateClusterValidatePolicy(obj *policyv1alpha1.ClusterValidatePolicy) (admission.Response, error) {
	for _, validateRule := range obj.Spec.ValidateRules {
		if validateRule.Template == nil || len(validateRule.RenderedCue) == 0 {
			continue
		}
		if err := p.cueManager.Validate([]byte(validateRule.RenderedCue), nil); err != nil {
			return admission.Denied(err.Error()), nil
		}
	}

	return admission.Allowed("ok"), nil
}

func (p *policyInterrupterImpl) validateOverridePolicy(objSpec *policyv1alpha1.OverridePolicySpec) (admission.Response, error) {
	for _, overrideRule := range objSpec.OverrideRules {
		if err := p.cueManager.Validate([]byte(overrideRule.Overriders.RenderedCue), nil); err != nil {
			return admission.Denied(err.Error()), nil
		}
	}

	return admission.Allowed("ok"), nil
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
