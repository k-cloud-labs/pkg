package templates

// ValidateTemplate -
var ValidateTemplate = `
{{define "BaseTemplate"}}
    {{- /*gotype:github.com/k-cloud-labs/pkg/utils/interrupter/model.ValidatePolicyRenderData*/ -}}
	{{if and (eq .Type "condition") (.Condition)}}
		{{template "ConditionTemplate" .Condition}}
	{{end}}
{{end}}

{{define "ConditionTemplate"}}
    {{- /*gotype:github.com/k-cloud-labs/pkg/utils/interrupter/model.ValidateCondition*/ -}}
{{if or (eq .Cond "In") (eq .Cond "NotIn")}}
import "list"
{{end}}
data: _ @tag(data)
object: data.object
oldObject: data.oldObject
{{if .ValueRef}}
	{{if or (eq .ValueRef.From "k8s") (eq .ValueRef.From "http") (eq .ValueRef.From "owner")}}
		{{.ValueRef.CueObjectKey}} : data.extraParams."{{.ValueRef.CueObjectKey}}"
	{{end}}
{{end}}
{{if .DataRef}}
    {{if or (eq .DataRef.From "k8s") (eq .DataRef.From "http")}}
        {{.DataRef.CueObjectKey}} : data.extraParams."{{.DataRef.CueObjectKey}}"
    {{end}}
{{end}}

validate:{
	{{if eq .Cond "NotExist"}}
		if {{.DataRef.CueObjectKey}}.{{.DataRef.Path}} == _|_ {
			{{template "reject" .}}
	{{else}}
		if {{.DataRef.CueObjectKey}}.{{.DataRef.Path}} != _|_ {
	    {{/*hanlde other cond*/}}
	    {{if ne .Cond "Exist"}}
	        {{template "estimate" .}}
	    {{end}}
	        {{template "reject" .}}
	    {{if ne .Cond "Exist"}}
			}
	        {{if eq .ValueType "ref"}}
				}
	        {{end}}
	    {{end}}
	{{end}}
	}
}
{{end}}

{{define "reject"}}
{{- /*gotype:github.com/k-cloud-labs/pkg/utils/interrupter/model.ValidateCondition*/ -}}
	valid: false
	reason: "{{.Message}}"
{{end}}


{{define "estimate"}}
    {{- /*gotype:github.com/k-cloud-labs/pkg/utils/interrupter/model.ValidateCondition*/ -}}
    {{if eq .ValueType "const"}}
        {{if eq .Cond "In"}}
			if list.Contains({{convertSliceValue .Value}}, {{.DataRef.CueObjectKey}}.{{.DataRef.Path}}) {
        {{else if eq .Cond "NotIn"}}
			if !list.Contains({{convertSliceValue .Value}}, {{.DataRef.CueObjectKey}}.{{.DataRef.Path}}) {
        {{else}}
	        {{if .ValueProcess}}
				if {{.DataRef.CueObjectKey}}.{{.DataRef.Path}} {{.Cond}} ({{convertConstValue .Value}} {{.ValueProcess.Operation}} {{.ValueProcess.OperationWith}}) {
		    {{else}}
				if {{.DataRef.CueObjectKey}}.{{.DataRef.Path}} {{.Cond}} {{convertConstValue .Value}} {
		    {{end}}
        {{end}}
    {{else}}
        {{if .ValueProcess}}
	        if {{.ValueRef.CueObjectKey}}.{{.ValueRef.Path}} != _|_ {
	            if {{.DataRef.CueObjectKey}}.{{.DataRef.Path}} {{.Cond}} ({{.ValueRef.CueObjectKey}}.{{.ValueRef.Path}} {{.ValueProcess.Operation}} {{.ValueProcess.OperationWith}}) {
        {{else}}
			if {{.ValueRef.CueObjectKey}}.{{.ValueRef.Path}} != _|_ {
				if {{.DataRef.CueObjectKey}}.{{.DataRef.Path}} {{.Cond}} {{.ValueRef.CueObjectKey}}.{{.ValueRef.Path}} {
	    {{end}}
    {{end}}
{{end}}
`
