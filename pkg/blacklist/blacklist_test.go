package blacklist

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewBlacklistIsEmpty(t *testing.T) {
	bl := New(zerolog.Logger{})
	assert.NotNil(t, bl.len())
	assert.Equal(t, 0, bl.len())
}

func TestAddToBlackList(t *testing.T) {
	bl := New(zerolog.Logger{})
	bl.Set("expanse")
	assert.Equal(t, 1, bl.len())
	bl.Set("outlander")
	assert.Equal(t, 2, bl.len())
	bl.Set("blueplanet ii")
	assert.Equal(t, 3, bl.len())
}

func TestAllAbsentFromEmptyBlacklist(t *testing.T) {
	bl := New(zerolog.Logger{})
	assert.Equal(t, false, bl.InList("expanse"))
	assert.Equal(t, false, bl.InList("elvis lives"))
}

func TestExistsInBlacklist(t *testing.T) {
	bl := New(zerolog.Logger{})
	bl.Set("default")
	assert.Equal(t, true, bl.InList("default"))
	assert.Equal(t, false, bl.InList("kube-system"))
	bl.Set("kube-system")
	assert.Equal(t, true, bl.InList("kube-system"))
	assert.Equal(t, false, bl.InList("haltandcatchfire"))
}

func TestSetBlacklistVariadic(t *testing.T) {
	bl := New(zerolog.Logger{})
	bl.Set("tom", "dick", "harry")
	assert.Equal(t, 3, bl.len())
	assert.Equal(t, true, bl.InList("tom"))
	assert.Equal(t, true, bl.InList("dick"))
	assert.Equal(t, true, bl.InList("harry"))
}

func TestOutputBlackListAsStringSlice(t *testing.T) {
	bl := New(zerolog.Logger{})
	values := []string{"dick", "harry", "tom"}
	bl.Set(values...)
	assert.Equal(t, values, bl.Values(), "The return slice is sorted")
}

func TestOutputBlacklistAsString(t *testing.T) {
	bl := New(zerolog.Logger{})
	bl.Set("tom", "dick", "harry")
	assert.Equal(t, "dick,harry,tom", bl.ValuesAsString(), "the output blacklist entries must be sorted alphabetically")
}
