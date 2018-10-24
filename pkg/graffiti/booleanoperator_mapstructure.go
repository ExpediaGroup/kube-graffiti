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
