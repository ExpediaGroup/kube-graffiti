package graffiti

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// createJSONPatch will generate a JSON patch for replacing an objects labels and/or annotations
// It is designed to replace the whole path in order to work around a bug in kubernetes that does not correctly
// unescape ~1 (/) in paths preventing annotation labels with slashes in them.
func (r Rule) createObjectPatch(obj metaObject, fm map[string]string) (string, error) {
	var patches []string

	if len(r.Additions.Labels) > 0 {
		modified, err := renderMapValues(r.Additions.Labels, fm)
		if err != nil {
			return "", err
		}

		if len(obj.Meta.Labels) == 0 {
			patches = append(patches, renderStringMapAsPatch("add", "/metadata/labels", modified))
		} else {
			patches = append(patches, renderStringMapAsPatch("replace", "/metadata/labels", mergeMaps(obj.Meta.Labels, modified)))
		}
	}

	if len(r.Additions.Annotations) > 0 {
		modified, err := renderMapValues(r.Additions.Annotations, fm)
		if err != nil {
			return "", err
		}

		if len(obj.Meta.Annotations) == 0 {
			patches = append(patches, renderStringMapAsPatch("add", "/metadata/annotations", modified))
		} else {
			patches = append(patches, renderStringMapAsPatch("replace", "/metadata/annotations", mergeMaps(obj.Meta.Annotations, modified)))
		}
	}

	return `[ ` + strings.Join(patches, ", ") + ` ]`, nil
}

// renderStringMapAsPatch builds a json patch string from operand, path and a map
func renderStringMapAsPatch(op, path string, m map[string]string) string {
	patch := `{ "op": "` + op + `", "path": "` + path + `", "value": { `
	var values []string
	for k, v := range m {
		values = append(values, `"`+k+`": "`+escapeString(v)+`"`)
	}
	patch = patch + strings.Join(values, ", ") + ` }}`
	return patch
}

func escapeString(s string) string {
	result := strings.Replace(s, "\n", "", -1)
	return strings.Replace(result, `"`, `\"`, -1)
}

func mergeMaps(sources ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, source := range sources {
		if len(source) > 0 {
			for k, v := range source {
				result[k] = v
			}
		}
	}

	return result
}

// renderMapValues - treat each map value as a template and render it using the data map as a context
func renderMapValues(src, data map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range src {
		if rendered, err := renderStringTemplate(v, data); err != nil {
			return result, err
		} else {
			result[k] = rendered
		}
	}
	return result, nil
}

// renderStringTemplate will treat the input string as a template and render with data as its context
// useful for allowing dynamically created values.
func renderStringTemplate(field string, data interface{}) (string, error) {
	tmpl, err := template.New("field").Parse(field)
	if err != nil {
		return "", fmt.Errorf("failed to parse field template: %v", err)
	}

	var b bytes.Buffer
	err = tmpl.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("error rendering template: %v", err)
	}
	return b.String(), nil
}
