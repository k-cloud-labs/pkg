package templatemanager

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"text/template"
)

// TemplateManager defines cue rules inside go templates which render the template to valid cue rule.
type TemplateManager interface {
	// Render - render go template with giver data and return result of render and an error
	Render(r io.Reader, data any) (result []byte, err error)
}

type templateManagerImpl struct {
	t    *template.Template
	name string
}

type PathOrFileName struct {
	// example: xx/xxx/*.tmpl
	PathPattern string
	// example: xx/xx/yy.tmpl
	FileName string
}

func NewTemplateManager(templateName string, pf *PathOrFileName, funcMap template.FuncMap) (tm TemplateManager, err error) {
	if pf == nil {
		return nil, errors.New("invalid params")
	}

	t := template.New("").Funcs(funcMap)

	if pf.PathPattern != "" {
		t, err = t.ParseGlob(pf.PathPattern)
		if err != nil {
			return
		}
		if templateName == "" {
			return nil, errors.New("must provide template name")
		}
	} else if pf.FileName != "" {
		t, err = t.ParseFiles(pf.FileName)
		if err != nil {
			return
		}
		if templateName == "" {
			templateName = pf.FileName[strings.LastIndex(pf.FileName, "/")+1:]
		}
	} else {
		return nil, errors.New("must provide path pattern or file path")
	}

	return &templateManagerImpl{
		t:    t,
		name: templateName,
	}, nil

}

func (t *templateManagerImpl) Render(reader io.Reader, data any) (result []byte, err error) {
	if reader == nil {
		return nil, nil
	}

	if data == nil {
		return nil, nil
	}

	buf := &bytes.Buffer{}
	err = t.t.ExecuteTemplate(buf, t.name, data)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
