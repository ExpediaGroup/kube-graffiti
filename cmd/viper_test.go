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

package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestUnknownConfigurationFieldsThrowAnError(t *testing.T) {
	var source = `elvis: "thank-you very much"`
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading into viper - it's perfectly valid to load anything")

	// assert that we can marshal the config into a Configuration struct
	_, err = unmarshalFromViperStrict()
	require.Error(t, err, "when unmarshaling into a strict Configuration it is, however, not ok to have unknown fields in viper")
}
