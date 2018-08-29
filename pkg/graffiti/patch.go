package graffiti

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	jsonpatch "github.com/cameront/go-jsonpatch"
	"github.com/getlantern/deepcopy"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

// createJSONPatch will generate a JSON patch of the difference between the source object and one with
// added labels and/or annotations
func (r Rule) createObjectPatch(obj metaObject, fm map[string]string) ([]byte, error) {
	mylog := log.ComponentLogger(componentName, "createObjectPatch")

	// make a deep copy of the request object and append any labels or annotations from the rule.
	var modified metaObject
	if err := deepcopy.Copy(&modified, &obj); err != nil {
		mylog.Error().Err(err).Msg("failed to deep copy the request object")
		return []byte{}, err
	}

	if len(r.Additions.Labels) > 0 {
		for k, v := range r.Additions.Labels {
			mylog.Debug().Str(k, v).Msg("adding label")
			if len(modified.Meta.Labels) == 0 {
				modified.Meta.Labels = make(map[string]string)
			}
			if rendered, err := renderFieldTemplate(v, fm); err != nil {
				return []byte{}, fmt.Errorf("failed to render label as a template: %v", err)
			} else {
				modified.Meta.Labels[k] = rendered
			}
		}
	}
	if len(r.Additions.Annotations) > 0 {
		for k, v := range r.Additions.Annotations {
			mylog.Debug().Str(k, v).Msg("adding annotation")
			if len(modified.Meta.Annotations) == 0 {
				modified.Meta.Annotations = make(map[string]string)
			}
			if rendered, err := renderFieldTemplate(v, fm); err != nil {
				return []byte{}, fmt.Errorf("failed to render annotation as a template: %v", err)
			} else {
				modified.Meta.Annotations[k] = rendered
			}
		}
	}

	return genericJSONPatch(obj, modified)
}

// renderFieldTemplate will treat the input string as a template and render with data as its context
// useful for allowing dynamically created values.
func renderFieldTemplate(field string, data interface{}) (string, error) {
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

func genericJSONPatch(src, dst interface{}) ([]byte, error) {
	mylog := log.ComponentLogger(componentName, "genericJSONPatch")

	// marshal the objects to json
	srcJSON, err := json.Marshal(src)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to marshal source object")
		return []byte{}, err
	}
	dstJSON, err := json.Marshal(dst)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to marshal destination object")
		return []byte{}, err
	}

	// unmarshal them back to map[string]interface{} objects
	var srcmap map[string]interface{}
	var dstmap map[string]interface{}
	if err := json.Unmarshal(srcJSON, &srcmap); err != nil {
		mylog.Error().Err(err).Msg("failed to unmarshal source json again")
		return []byte{}, err
	}
	if err := json.Unmarshal(dstJSON, &dstmap); err != nil {
		mylog.Error().Err(err).Msg("failed to unmarshal source json again")
		return []byte{}, err
	}

	// generate a patch and return as json
	patch, err := jsonpatch.MakePatch(srcmap, dstmap)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to make json patch")
		return []byte{}, err
	}

	json, err := patch.MarshalJSON()
	if err != nil {
		mylog.Error().Err(err).Msg("failed to marshal patch json")
		return []byte{}, err
	}
	return json, nil
}
