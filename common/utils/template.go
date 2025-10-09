package utils

import (
	"bytes"
	"text/template"
)

func RenderTemplate(tpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
