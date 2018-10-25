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
	"fmt"
	"regexp"
	"strings"

	jsonpatch "github.com/cameront/go-jsonpatch"
	"github.com/rs/zerolog"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

// Payload contains the actions that we would like to perform when rule matches an object, such as
// label/annotation additions or deletions, a patch or a block.
type Payload struct {
	Additions Additions `mapstructure:"additions" yaml:"additions,omitempty"`
	Deletions Deletions `mapstructure:"deletions" yaml:"deletions,omitempty"`
	Block     bool      `mapstructure:"block" yaml:"block,omitempty"`
	JSONPatch string    `mapstructure:"json-patch" yaml:"json-patch,omitempty"`
}

// Additions contains the additional fields that we want to insert into the object
// This type is directly marshalled from config and so has mapstructure tags
type Additions struct {
	Annotations map[string]string `mapstructure:"annotations" yaml:"annotations,omitempty"`
	Labels      map[string]string `mapstructure:"labels" yaml:"labels,omitempty"`
}

// Deletions contains the names of labels or annotations which you wish to remove
type Deletions struct {
	Annotations []string `mapstructure:"annotations" yaml:"annotations,omitempty"`
	Labels      []string `mapstructure:"labels" yaml:"labels,omitempty"`
}

func (p Payload) paintObject(object metaObject, fm map[string]string, logger zerolog.Logger) (patch []byte, err error) {
	mylog := logger.With().Str("func", "paintObject").Logger()

	// a block takes precedence over JSONPatch, Additions, Deletions...
	if p.Block {
		mylog.Debug().Msg("payload contains a block")
		return []byte("BLOCK"), nil
	}

	// if the user provided a patch then just use that...
	if p.JSONPatch != "" {
		mylog.Debug().Str("patch", p.JSONPatch).Msg("payload contains user provided patch")
		return []byte(p.JSONPatch), nil
	}

	// create a patch for additions + deletions
	var patchString string
	if p.containsAdditions() || p.containsDeletions() {
		mylog.Debug().Str("patch", p.JSONPatch).Msg("payload contains additions or deletions")
		patchString, err = p.processMetadataAdditionsDeletions(object, fm)
		if err != nil {
			return nil, fmt.Errorf("could not create json patch: %v", err)
		}
	}

	if patchString == "" {
		mylog.Info().Msg("paint resulted in no patch")
		return nil, nil
	}

	mylog.Info().Str("patch", patchString).Msg("created json patch")
	return []byte(patchString), nil
}

func (p Payload) containsAdditions() bool {
	if len(p.Additions.Labels) == 0 && len(p.Additions.Annotations) == 0 {
		return false
	}
	return true
}

func (p Payload) containsDeletions() bool {
	if len(p.Deletions.Labels) == 0 && len(p.Deletions.Annotations) == 0 {
		return false
	}
	return true
}

// processMetadataAdditionsDeletions will generate a JSON patch for replacing an objects labels and/or annotations
// It is designed to replace the whole path in order to work around a bug in kubernetes that does not correctly
// unescape ~1 (/) in paths preventing annotation labels with slashes in them.
func (p Payload) processMetadataAdditionsDeletions(obj metaObject, fm map[string]string) (string, error) {
	mylog := log.ComponentLogger(componentName, "processMetadataAdditionsDeletions")
	var patches []string

	op, err := createPatchOperand(obj.Meta.Labels, p.Additions.Labels, fm, p.Deletions.Labels, "/metadata/labels")
	if err != nil {
		return "", err
	}
	if op != "" {
		mylog.Debug().Str("operand", op).Msg("created patch operand")
		patches = append(patches, op)
	}

	op, err = createPatchOperand(obj.Meta.Annotations, p.Additions.Annotations, fm, p.Deletions.Annotations, "/metadata/annotations")
	if err != nil {
		return "", err
	}
	if op != "" {
		mylog.Debug().Str("operand", op).Msg("created patch operand")
		patches = append(patches, op)
	}

	if len(patches) == 0 {
		return "", nil
	}
	return `[ ` + strings.Join(patches, ", ") + ` ]`, nil
}

// Validate can be used by clients of payload to validate that its syntax and contents are correct.
func (p Payload) validate() error {
	var payloadTypes = 0
	var hasJSONPatch bool
	var hasAdditionsDeletions bool

	if p.Block {
		payloadTypes++
	}
	if p.JSONPatch != "" {
		hasJSONPatch = true
		payloadTypes++
	}
	if len(p.Additions.Labels) != 0 || len(p.Additions.Annotations) != 0 || len(p.Deletions.Labels) != 0 || len(p.Deletions.Annotations) != 0 {
		hasAdditionsDeletions = true
		payloadTypes++
	}
	if payloadTypes == 0 {
		return fmt.Errorf("a rule payload must specify either additions/deletions, a json-patch, or a block")
	}
	if payloadTypes > 1 {
		return fmt.Errorf("a rule payload can only specify additions/deletions, or a json-patch or a block, but not a combination of them")
	}

	if hasJSONPatch {
		return validateJSONPatch(p.JSONPatch)
	}
	if hasAdditionsDeletions {
		return validateAdditionsDeletions(p.Additions, p.Deletions)
	}

	return nil
}

// validateJSONPatch uses the jsonpatch go package to parse the user supplied patch
// and return an error if the patch syntax is invalid.
func validateJSONPatch(p string) error {
	fmt.Printf("validating json patch: %s\n", p)
	if _, err := jsonpatch.FromString(p); err != nil {
		return fmt.Errorf("invalid json-patch: %v", err)
	}
	return nil
}

// validateAdditionsDeletions validates all additions and deletions fields are valid if they are specified.
func validateAdditionsDeletions(add Additions, del Deletions) (err error) {
	if len(add.Labels) > 0 {
		if err = validateAdditionsLabels(add.Labels); err != nil {
			return err
		}
	}
	if len(add.Annotations) > 0 {
		if err = validateAdditionsAnnotations(add.Annotations); err != nil {
			return err
		}
	}
	if len(del.Labels) > 0 {
		if err = validateDeletionsKeys(del.Labels); err != nil {
			return err
		}
	}
	if len(del.Annotations) > 0 {
		if err = validateDeletionsKeys(del.Annotations); err != nil {
			return err
		}
	}
	return nil
}

// validateAdditionsLabels knows how validate kubernetes labels
func validateAdditionsLabels(labels map[string]string) error {
	// validate all additions labels using kubernetes validation methods
	templateRegex := regexp.MustCompile(`\{\{.*\}\}`)
	for k, v := range labels {
		if errorList := utilvalidation.IsQualifiedName(k); len(errorList) != 0 {
			return fmt.Errorf("invalid additions: invalid label key \"%s\": %s", k, strings.Join(errorList, "; "))
		}
		if templateRegex.MatchString(v) {
			continue
		} else {
			if errorList := utilvalidation.IsValidLabelValue(v); len(errorList) != 0 {
				return fmt.Errorf("invalid additions: invalid label value \"%s\": %s", v, strings.Join(errorList, "; "))
			}
		}
	}
	return nil
}

// validateAdditionsAnnotations knows how validate kubernetes annotations
func validateAdditionsAnnotations(annotations map[string]string) error {
	// validate all additions annotations by using kubernetes validation methods
	path := field.NewPath("metadata.annotations")
	if errorList := apivalidation.ValidateAnnotations(annotations, path); len(errorList) != 0 {
		var info []string
		for _, errorPart := range errorList.ToAggregate().Errors() {
			info = append(info, errorPart.Error())
		}
		return fmt.Errorf("invalid additions: invalid annotations: %s", strings.Join(info, "; "))
	}
	return nil
}

// validateDeletionsKeys checks that either label or annotation keys are valid QualifiedDomainNames
func validateDeletionsKeys(labels []string) error {
	for _, v := range labels {
		if errorList := utilvalidation.IsQualifiedName(v); len(errorList) != 0 {
			return fmt.Errorf("invalid deletions: invalid key: %s", strings.Join(errorList, "; "))
		}
	}
	return nil
}
