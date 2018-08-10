package graffiti

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyObject(t *testing.T) {
	_, err := makeFieldMap([]byte{})
	require.Error(t, err)
	assert.Equal(t, "no fields found", err.Error())

}

func TestTopLevelObjectMustBeAMap(t *testing.T) {
	validJSON := `[ "apple", "orange", "banana" ]`
	_, err := makeFieldMap([]byte(validJSON))
	assert.Error(t, err)
	assert.Equal(t, "failed to unmarshal object: json: cannot unmarshal array into Go value of type map[string]interface {}", err.Error())
}

func TestBaseTypesAsStrings(t *testing.T) {
	// when creating a fieldmap the following json types are converted to strings

	// strings
	testJSON := `{ "test": "dave" }`
	fm, err := makeFieldMap([]byte(testJSON))
	require.NoError(t, err)
	assert.Equal(t, "dave", fm["test"])

	// ints
	testJSON = `{ "test": 100 }`
	fm, err = makeFieldMap([]byte(testJSON))
	require.NoError(t, err)
	assert.Equal(t, "100", fm["test"])

	// floats
	testJSON = `{ "test": 63.333392 }`
	fm, err = makeFieldMap([]byte(testJSON))
	require.NoError(t, err)
	assert.Equal(t, "63.333392", fm["test"])

	// bools
	testJSON = `{ "test": true }`
	fm, err = makeFieldMap([]byte(testJSON))
	require.NoError(t, err)
	assert.Equal(t, "true", fm["test"])
}

func TestSlicesAreReferencedByIndex(t *testing.T) {
	testJSON := `{ "test": [ "dave", 100, 63.49, true ] }`
	fm, err := makeFieldMap([]byte(testJSON))
	require.NoError(t, err)

	assert.Equal(t, "dave", fm["test.0"])
	assert.Equal(t, "100", fm["test.1"])
	assert.Equal(t, "63.49", fm["test.2"])
	assert.Equal(t, "true", fm["test.3"])
}

func TestMapsAreReferencedByKey(t *testing.T) {
	testJSON := `{ "test": { "band": "Queen", "singer": "Freddie Mercury", "status": "legend" }}`
	fm, err := makeFieldMap([]byte(testJSON))
	require.NoError(t, err)

	assert.Equal(t, "Queen", fm["test.band"])
	assert.Equal(t, "Freddie Mercury", fm["test.singer"])
	assert.Equal(t, "legend", fm["test.status"])
}

func TestComplexObject(t *testing.T) {
	var testJSON = `{  
		"metadata":{  
			"name":"test-namespace",
			"creationTimestamp":null,
			"labels":{
				"author": "david",
				"group": "runtime"
			},
			"annotations":{
				"level": "v.special",
				"prometheus.io/path": "/metrics"
			}
		},
		"spec":{},
		"status":{  
			"phase":"Active"
		}
	 }`

	fm, err := makeFieldMap([]byte(testJSON))
	require.NoError(t, err)

	assert.Equal(t, "test-namespace", fm["metadata.name"])
	assert.Equal(t, "david", fm["metadata.labels.author"])
	assert.Equal(t, "v.special", fm["metadata.annotations.level"])
	assert.Equal(t, "Active", fm["status.phase"])
}
