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

	"github.com/HotelsDotCom/kube-graffiti/pkg/log"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/fields"
	labels "k8s.io/apimachinery/pkg/labels"
)

// Matchers manages the rules of matching an object
// This type is directly marshalled from config and so has mapstructure tags
type Matchers struct {
	LabelSelectors  []string        `mapstructure:"label-selectors" yaml:"label-selectors,omitempty"`
	FieldSelectors  []string        `mapstructure:"field-selectors" yaml:"field-selectors,omitempty"`
	BooleanOperator BooleanOperator `mapstructure:"boolean-operator" yaml:"boolean-operator,omitempty"`
}

func (m Matchers) validate(rulelog zerolog.Logger) error {
	// all label selectors must be valid...
	if len(m.LabelSelectors) > 0 {
		for _, selector := range m.LabelSelectors {
			if err := ValidateLabelSelector(selector); err != nil {
				rulelog.Error().Str("label-selector", selector).Msg("matcher contains an invalid label selector")
				return fmt.Errorf("matcher contains an invalid label selector '%s': %v", selector, err)
			}
		}
	}

	// all field selectors must also be valid...
	if len(m.FieldSelectors) > 0 {
		for _, selector := range m.FieldSelectors {
			if err := validateFieldSelector(selector); err != nil {
				rulelog.Error().Str("field-selector", selector).Msg("matcher contains an invalid field selector")
				return fmt.Errorf("matcher contains invalid field selector '%s': %v", selector, err)
			}
		}
	}
	return nil
}

// validateLabelSelector checks that a label selector parses correctly and is used when validating config
func ValidateLabelSelector(selector string) error {
	if _, err := labels.Parse(selector); err != nil {
		return err
	}
	return nil
}

// validateFieldSelector checks that a field selector parses correctly and is used when validating config
func validateFieldSelector(selector string) error {
	if _, err := fields.ParseSelector(selector); err != nil {
		return err
	}
	return nil
}

func (m Matchers) matches(obj metaObject, fm map[string]string, mylog zerolog.Logger) (match bool, err error) {
	var labelMatches, fieldMatches bool
	if len(m.LabelSelectors) == 0 && len(m.FieldSelectors) == 0 {
		mylog.Debug().Msg("rule does not contain any label or field selectors so it matches ALL")
		return true, nil
	}

	// match against all of the label selectors
	mylog.Debug().Int("count", len(m.LabelSelectors)).Msg("matching against label selectors")
	labelMatches, err = m.matchLabelSelectors(obj)
	if err != nil {
		return false, err
	}

	// test if we match any field selectors
	mylog.Debug().Int("count", len(m.FieldSelectors)).Msg("matching against field selectors")
	fieldMatches, err = m.matchFieldSelectors(fm)
	if err != nil {
		return false, err
	}

	// Combine selector booleans and decide to paint object or not
	descisonLog := mylog.With().Int("label-selectors-length", len(m.LabelSelectors)).Bool("labels-matched", labelMatches).Int("field-selector-length", len(m.FieldSelectors)).Bool("fields-matched", fieldMatches).Logger()
	switch m.BooleanOperator {
	case AND:
		descisonLog.Debug().Str("boolean-operator", "AND").Msg("performed label-selector AND field-selector")
		return (len(m.LabelSelectors) == 0 || labelMatches) && (len(m.FieldSelectors) == 0 || fieldMatches), nil
	case OR:
		descisonLog.Debug().Str("boolean-operator", "OR").Msg("performed label-selector OR field-selector")
		return (len(m.LabelSelectors) != 0 && labelMatches) || (len(m.FieldSelectors) != 0 && fieldMatches), nil
	case XOR:
		descisonLog.Debug().Str("boolean-operator", "XOR").Msg("performed label-selector XOR field-selector")
		return labelMatches != fieldMatches, nil
	default:
		descisonLog.Fatal().Str("boolean-operator", "UNKNOWN").Msg("Boolean Operator isn't one of AND, OR, XOR")
		return false, fmt.Errorf("Boolean Operator isn't one of AND, OR, XOR")
	}
}

func (m Matchers) matchLabelSelectors(object metaObject) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchLabelSelectors")
	// test if we matched any of the label selectors
	if len(m.LabelSelectors) != 0 {
		sourceLabels := make(map[string]string)
		// make it so we can use name and namespace as label selectors
		sourceLabels["name"] = object.Meta.Name
		sourceLabels["namespace"] = object.Meta.Namespace
		for k, v := range object.Meta.Labels {
			sourceLabels[k] = v
		}

		for _, selector := range m.LabelSelectors {
			mylog.Debug().Str("label-selector", selector).Msg("testing label selector")
			selectorMatch, err := MatchLabelSelector(selector, sourceLabels)
			if err != nil {
				return false, err
			}
			if selectorMatch {
				mylog.Debug().Str("label-selector", selector).Msg("selector matches, will modify object")
				return true, nil
			}
		}
	}
	return false, nil
}

// matchLabelSelector will apply a kubernetes labels.Selector to a map[string]string and return a matched bool and error.
// It is exported so that it can be used in 'existing' package for processing namespace selectors.
func MatchLabelSelector(selector string, target map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "MatchLabelSelector")
	selLog := mylog.With().Str("selector", selector).Logger()

	realSelector, err := labels.Parse(selector)
	if err != nil {
		selLog.Error().Err(err).Msg("could not parse selector")
		return false, err
	}

	set := labels.Set(target)
	if !realSelector.Matches(set) {
		selLog.Debug().Msg("selector does not match")
		return false, nil
	}
	selLog.Debug().Msg("selector matches")
	return true, nil
}

func (m Matchers) matchFieldSelectors(fm map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchFieldSelectors")
	if len(m.FieldSelectors) != 0 {
		for _, selector := range m.FieldSelectors {
			mylog.Debug().Str("field-selector", selector).Msg("testing field selector")
			selectorMatch, err := matchFieldSelector(selector, fm)
			if err != nil {
				return false, err
			}
			if selectorMatch {
				mylog.Debug().Str("field-selector", selector).Msg("selector matches, will modify object")
				return true, nil
			}
		}
	}
	return false, nil
}

// matchSelector will apply a kubernetes labels.Selector to a map[string]string and return a matched bool and error.
func matchFieldSelector(selector string, target map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchFieldSelector")
	selLog := mylog.With().Str("selector", selector).Logger()
	realSelector, err := fields.ParseSelector(selector)
	if err != nil {
		selLog.Error().Err(err).Msg("could not parse selector")
		return false, err
	}

	set := labels.Set(target)
	if !realSelector.Matches(set) {
		selLog.Debug().Msg("selector does not match")
		return false, nil
	}
	selLog.Debug().Msg("selector matches")
	return true, nil
}
