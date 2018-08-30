package jsonpatch

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMakePatch_nop ensures that MakePatch returns an empty patch when the
// input equals the output.
func TestMakePatch_nop(t *testing.T) {
	for i, test := range []struct {
		src string
	}{
		{ // simple map
			`{"a":1,"b":2}`,
		},
		{ // nested map
			`{"a":{"b":"c", "d":"e"}}`,
		},
		{ // array
			`{"a":[0, 1, 2, 3]}`,
		},
	} {
		index := fmt.Sprintf("test %d", i)
		docA := getMapDoc(test.src)
		docB := getMapDoc(test.src)
		patch, err := MakePatch(docA, docB)
		assert.Nil(t, err, index)
		assert.Equal(t, len(patch.Operations), 0, index)
	}
}

// TestMakePatch ensures that patches cleanly apply to the original documents
// and produce a document equivalent to the target.
func TestMakePatch(t *testing.T) {
	for i, test := range []struct {
		src, dst string
	}{
		{ // simple map operations.
			`{"a":1,"b":2}`,
			`{"a":2,"b":1}`,
		},
		{ // nested map operations
			`{"this":{"is":"my", "document":"sir"}}`,
			`{"this":{"document":"my", "is":"sir", "now":{"go":"away!"}}}`,
		},
		{ // array operations
			`{"a":[0, 1, 2, 3]}`,
			`{"a":[1, 2, 4, "hi"]}`,
		},
		/* BUG: this test panics
		{ // nested array operations
			`{"a":[0, [1, 2, 3], 5, 2]}`,
			`{"a":[1, [1, 0, 2, 3, 5], "x", 2]}`,
		},
		*/
	} {
		index := fmt.Sprintf("test %d", i)
		docA := getMapDoc(test.src)
		docB := getMapDoc(test.dst)
		patch, err := MakePatch(docA, docB)
		assert.Nil(t, err, index)
		err = patch.Apply(&docA)
		assert.Nil(t, err, index)
		assert.Equal(t, docA, docB, index)
	}
}

func TestPatchMarshalAndUnmarshal(t *testing.T) {
	for i, test := range []struct {
		src  string
		dest Patch
	}{
		{ // empty
			`[]`,
			Patch{Operations: []PatchOperation{}},
		},
		{ // value is ommitted
			`[{"from":"/foo","op":"move","path":"/foo2"}]`,
			Patch{Operations: []PatchOperation{PatchOperation{Op: Move, From: "/foo", Path: "/foo2"}}},
		},
		{ // from is ommitted
			`[{"op":"replace","path":"/foo","value":"foo"}]`,
			Patch{Operations: []PatchOperation{PatchOperation{Op: Replace, Path: "/foo", Value: "foo"}}},
		},
		{ // value and from are ommitted
			`[{"op":"remove","path":"/foo"}]`,
			Patch{Operations: []PatchOperation{PatchOperation{Op: Remove, Path: "/foo"}}},
		},
	} {
		index := fmt.Sprintf("test %d", i)

		var p Patch
		err := json.Unmarshal([]byte(test.src), &p)
		assert.Nil(t, err, index)
		assert.Equal(t, p, test.dest)

		jp, err := json.Marshal(p)
		assert.Nil(t, err, index)
		assert.Equal(t, string(jp), test.src)
	}
}

func TestApplyPatchFromString(t *testing.T) {
	doc := getMapDoc(`{"foo": "bar"}`)

	patchOp, err := FromString(`[{"op": "add", "path": "/baz", "value": "qux"}]`)
	assert.Nil(t, err)
	patchOp.Apply(&doc)
	val, found := doc["baz"]
	assert.True(t, found)
	assert.Equal(t, "qux", val.(string))
}

func TestLcs(t *testing.T) {
	pairA, pairB := longestCommonSubseq(slice(1, 2, 3, 4), slice(0, 1, 2, 3, 5))
	assert.Equal(t, intPair{0, 3}, *pairA)
	assert.Equal(t, intPair{1, 4}, *pairB)

	pairA, pairB = longestCommonSubseq(slice(1, 3, 5), slice(0, 1, 2, 3, 4, 5, 6))
	assert.Equal(t, intPair{2, 3}, *pairA)
	assert.Equal(t, intPair{5, 6}, *pairB)
}

func TestSplitByCommonSeq(t *testing.T) {
	node := splitByCommonSeq(slice(0, 1, 2, 3), slice(1, 2, 4, 5), &intPair{0, -1}, &intPair{0, -1})
	assert.Nil(t, node.left)
	assert.Nil(t, node.right)

	// Left subtree
	assert.NotNil(t, node.leftPtr)
	assert.Equal(t, intPair{0, 1}, *node.leftPtr.left)
	assert.Nil(t, node.leftPtr.leftPtr)
	assert.Nil(t, node.leftPtr.rightPtr)
	// Right subtree
	assert.NotNil(t, node.rightPtr)
	assert.Equal(t, intPair{3, 4}, *node.rightPtr.left)
	assert.Equal(t, intPair{2, 4}, *node.rightPtr.right)
	assert.Nil(t, node.rightPtr.rightPtr)
	assert.Nil(t, node.rightPtr.leftPtr)
}

func slice(args ...interface{}) []interface{} {
	s := []interface{}{}
	s = append(s, args...)
	return s
}
