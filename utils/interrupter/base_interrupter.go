package interrupter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils/interrupter/model"
	"github.com/k-cloud-labs/pkg/utils/templatemanager"
)

type baseInterrupter struct {
	overrideTemplateManager templatemanager.TemplateManager
	validateTemplateManager templatemanager.TemplateManager
	cueManager              templatemanager.CueManager
}

func (i *baseInterrupter) OnMutating(obj, oldObj *unstructured.Unstructured) ([]jsonpatchv2.JsonPatchOperation, error) {
	// do nothing, need override this method
	return nil, nil
}

func (i *baseInterrupter) OnValidating(obj, oldObj *unstructured.Unstructured) error {
	// do nothing, need override this method
	return nil
}

func NewBaseInterrupter(otm, vtm templatemanager.TemplateManager, cm templatemanager.CueManager) PolicyInterrupter {
	return &baseInterrupter{
		overrideTemplateManager: otm,
		validateTemplateManager: vtm,
		cueManager:              cm,
	}
}

func (i *baseInterrupter) renderAndFormat(data any) (b []byte, err error) {
	switch tmpl := data.(type) {
	case *policyv1alpha1.OverrideRuleTemplate:
		mrd := model.OverrideRulesToOverridePolicyRenderData(tmpl)
		b, err := i.overrideTemplateManager.Render(mrd)
		if err != nil {
			return nil, err
		}

		return i.cueManager.Format(trimBlankLine(b))
	case *policyv1alpha1.ValidateRuleTemplate:
		vrd := model.ValidateRulesToValidatePolicyRenderData(tmpl)
		b, err := i.validateTemplateManager.Render(vrd)
		if err != nil {
			return nil, err
		}

		return i.cueManager.Format(trimBlankLine(b))
	}

	return nil, errors.New("unknown data type")
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

func convertToPolicy(u *unstructured.Unstructured, data any) error {
	klog.V(4).Infof("convertToPolicy, obj=%v", u)
	b, err := u.MarshalJSON()
	if err != nil {
		return err
	}

	return json.Unmarshal(b, data)
}
