package graffiti

import (
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// StringToBooleanOperatorFunc allows mapstructure to map string representations of
// BooleanOperators to their enum type values
func StringToBooleanOperatorFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		var testBool BooleanOperator
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(testBool) {
			return data, nil
		}
		// Convert it by parsing
		return BooleanOperatorString(data.(string))
	}
}
