package templatemanager

import (
	"bytes"
	"errors"
	"text/template"
)

// TemplateManager defines cue rules inside go templates which render the template to valid cue rule.
type TemplateManager interface {
	// Render - render go template with giver data and return result of render and an error
	Render(data any) (result []byte, err error)
}

type templateManagerImpl struct {
	t  *template.Template
	ts *TemplateSource
}

// TemplateSource sets template source content, users can set single file path, pattern for multi files or
//
//	body of file content. But only can set one of three fields, if there are more than one field be set then will use
//	the first one only.
type TemplateSource struct {
	// example: xx/xxx/*.tmpl
	PathPattern string
	// example: xx/xx/yy.tmpl
	FilePath string
	// also can set template content via this field
	Content string
	// TemplateName name of template, if using multi defined template then must specify template name.
	TemplateName string
}

func (ts *TemplateSource) validate() error {
	if ts == nil {
		return errors.New("must provide template source")
	}

	if len(ts.PathPattern) == 0 && len(ts.FilePath) == 0 && len(ts.Content) == 0 {
		return errors.New("must provide one of template source")
	}

	if len(ts.TemplateName) == 0 {
		return errors.New("must provide template name")
	}

	return nil
}

func (ts *TemplateSource) parse(t1 *template.Template) (t *template.Template, err error) {
	if len(ts.PathPattern) > 0 {
		return t1.ParseGlob(ts.PathPattern)
	}

	if len(ts.FilePath) > 0 {
		return t1.ParseFiles(ts.FilePath)
	}

	return t1.Parse(ts.Content)
}

// NewTemplateManager init a manager based on given parameters.
func NewTemplateManager(ts *TemplateSource, funcMap template.FuncMap) (tm TemplateManager, err error) {
	if err = ts.validate(); err != nil {
		return nil, err
	}

	t := template.New("").Funcs(funcMap)
	t, err = ts.parse(t)
	if err != nil {
		return nil, err
	}

	return &templateManagerImpl{
		t:  t,
		ts: ts,
	}, nil
}

func (t *templateManagerImpl) Render(data any) (result []byte, err error) {
	if data == nil {
		return nil, errors.New("data must be a pointer")
	}

	buf := &bytes.Buffer{}
	err = t.t.ExecuteTemplate(buf, t.ts.TemplateName, data)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
