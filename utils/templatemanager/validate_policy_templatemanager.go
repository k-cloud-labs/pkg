package templatemanager

import (
	"encoding/json"
	"fmt"
	"text/template"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

// NewValidateTemplateManager init validate policy template manager
func NewValidateTemplateManager(filePath string) (TemplateManager, error) {
	t, err := NewTemplateManager(
		&TemplateSource{
			FilePath:     filePath,
			TemplateName: "BaseTemplate",
		}, template.FuncMap{
			"convertConstValue": func(v interface{}) string {
				val := v.(*policyv1alpha1.CustomTypes)
				if val.String != nil {
					return fmt.Sprintf("\"%s\"", *val.String)
				}

				return fmt.Sprintf("%v", val.Value())
			},

			"convertSliceValue": func(v interface{}) string {
				val := v.(*policyv1alpha1.CustomTypes)
				if slice := val.GetSlice(); len(slice) > 0 {
					b, _ := json.Marshal(slice)
					return string(b)
				}

				return "[]"
			},
		})
	if err != nil {
		return nil, err
	}

	return t, nil
}
