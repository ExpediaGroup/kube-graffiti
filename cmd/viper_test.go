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
