/*
Copyright (C) 2018 Expedia Group.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package graffiti

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"
)

func createPatchOperand(src, add, fm map[string]string, del []string, path string) (string, error) {
	modified := mergeMaps(src)

	// first process any additions into modified map
	if len(add) > 0 {
		rendered, err := renderMapValues(add, fm)
		if err != nil {
			return "", err
		}
		modified = mergeMaps(src, rendered)
	}

	// then process any deletions into modified map
	if len(del) > 0 {
		for _, d := range del {
			if _, ok := modified[d]; ok {
				delete(modified, d)
			}
		}
	}

	// don't produce a patch when there are no changes
	if reflect.DeepEqual(src, modified) {
		return "", nil
	}

	// do not return a patch where there are not any changes
	if len(src) == 0 && len(modified) == 0 {
		return "", nil
	}

	// we are left with new values, we need to either add a new path or replace it.
	if len(src) == 0 {
		return renderStringMapAsPatch("add", path, modified), nil
	}
	return renderStringMapAsPatch("replace", path, modified), nil
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
		for k, v := range source {
			result[k] = v
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
