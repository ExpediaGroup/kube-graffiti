package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathSimple(t *testing.T) {
	assert.Equal(t, pathPrefix+"/testing123", sanitizePath("testing123"), "should escape illegal url characters and add prefix")
}

func TestPathWithUnderscoresAndDashes(t *testing.T) {
	assert.Equal(t, pathPrefix+"/test-with_underscores_and-dashes", sanitizePath("test-with_underscores_and-dashes"), "should escape illegal url characters and add prefix")
}

func TestPathWithSymbols(t *testing.T) {
	assert.Equal(t, pathPrefix+"/test%21@%23$%25%5E&%2Aexample.com", sanitizePath("test!@#$%^&*example.com"), "should escape illegal url characters and add prefix")
}

func TestPathWithSlashes(t *testing.T) {
	assert.Equal(t, pathPrefix+"/%2Ftest%2Fpath%2Fwith%2Fslashes", sanitizePath("/test/path/with/slashes"), "should escape illegal url characters and add prefix")
}
