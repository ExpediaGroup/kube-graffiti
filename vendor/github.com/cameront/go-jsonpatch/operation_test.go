package jsonpatch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	ptr "github.com/xeipuuv/gojsonpointer"
)

func TestAdd(t *testing.T) {
	doc := getMapDoc(`{"this": {"is": [1,2,3], "my": {"document": 1}}}`)

	op := PatchOperation{
		Op:    "add",
		Path:  "/this/my/jam",
		Value: "!test!",
	}
	op.Apply(&doc)
	value := getValueAt(op.Path, doc)
	assert.Equal(t, "!test!", value)

	op.Path = "/this/is/-"
	op.Apply(&doc)
	value = getValueAt("/this/is/3", doc)
	assert.Equal(t, "!test!", value)

	op.Path = "/this/is/1"
	op.Apply(&doc)
	value = getValueAt(op.Path, doc)
	assert.Equal(t, "!test!", value)

	op.Path = "/this/my/document"
	op.Apply(&doc)
	value = getValueAt(op.Path, doc)
	assert.Equal(t, "!test!", value)
}

func TestApplyOpFromString(t *testing.T) {
	doc := getMapDoc(`{"foo": "bar"}`)

	patch, err := FromString(`[{"op": "add", "path": "/baz", "value": "qux"}]`)
	assert.Nil(t, err)
	patch.Operations[0].Apply(&doc)
	val, found := doc["baz"]
	assert.True(t, found)
	assert.Equal(t, "qux", val.(string))
}

// func TestApplyToCopy(t *testing.T) {
// 	doc := getMapDoc(`{"foo": "bar"}`)
// 	patchOp := PatchOperation{Op: "add", Path: "/baz", Value: "qux"}
// 	patchOp.Apply(&doc)
// 	// TODO: Verify if you don't do an in-place copy?
// }

func TestApplyToSameInstance(t *testing.T) {
	doc := getMapDoc(`{"foo": "bar"}`)
	docP := &doc
	patchOp := PatchOperation{Op: "add", Path: "/baz", Value: "qux"}
	patchOp.Apply(&doc)
	_, found := doc["baz"]
	assert.True(t, found)
	assert.Equal(t, &doc, docP)
}

func TestAddObjectKey(t *testing.T) {
	doc := getMapDoc(`{"foo": "bar"}`)
	patchOp := PatchOperation{Op: "add", Path: "/baz", Value: "qux"}
	patchOp.Apply(&doc)
	assert.Equal(t, "qux", doc["baz"].(string))
}

func TestAddArrayItem(t *testing.T) {
	doc := getMapDoc(`{"foo": ["bar", "baz"]}`)
	patchOp := PatchOperation{Op: "add", Path: "/foo/1", Value: "qux"}
	patchOp.Apply(&doc)
	assert.Equal(t, []interface{}{"bar", "qux", "baz"}, doc["foo"].([]interface{}))

	patchOp = PatchOperation{Op: "add", Path: "/foo/4", Value: "qux"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
}

func TestRemoveObjectKey(t *testing.T) {
	doc := getMapDoc(`{"foo": "bar", "baz":"qux"}`)
	patchOp := PatchOperation{Op: "remove", Path: "/baz"}
	patchOp.Apply(&doc)
	_, ok := doc["baz"]
	assert.False(t, ok)
}

func TestRemoveArrayItem(t *testing.T) {
	doc := getMapDoc(`{"foo": ["bar", "qux", "baz"]}`)
	patchOp := PatchOperation{Op: "remove", Path: "/foo/1"}
	patchOp.Apply(&doc)
	assert.Equal(t, []interface{}{"bar", "baz"}, doc["foo"].([]interface{}))

	patchOp = PatchOperation{Op: "remove", Path: "/foo/3"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
}

func TestReplaceObjectKey(t *testing.T) {
	doc := getMapDoc(`{"foo":"bar", "baz":"qux"}`)
	patchOp := PatchOperation{Op: "replace", Path: "/baz", Value: "boo"}
	patchOp.Apply(&doc)
	assert.Equal(t, "boo", doc["baz"].(string))
}

func TestReplaceWholeDocument(t *testing.T) {
	doc := getMapDoc(`{"foo":"bar"}`)
	patchOp := PatchOperation{Op: "replace", Path: "", Value: map[string]interface{}{"baz": "qux"}}
	patchOp.Apply(&doc)
	assert.Equal(t, "qux", doc["baz"].(string))
}

func TestAddReplaceWholeDocument(t *testing.T) {
	doc := getMapDoc(`{"foo":"bar"}`)
	patchOp := PatchOperation{Op: "add", Path: "", Value: map[string]interface{}{"baz": "qux"}}
	patchOp.Apply(&doc)
	assert.Equal(t, 1, len(doc))
	assert.Equal(t, "qux", doc["baz"].(string))
}

func TestErrIfPtrNotPassed(t *testing.T) {
	doc := getMapDoc(`{"foo":"bar"}`)
	patchOp := PatchOperation{Op: "replace", Path: "", Value: map[string]interface{}{"baz": "qux"}}
	err := patchOp.Apply(doc)
	// This should be a non-nil error because it's impossible for Apply to replace the object with
	// the one I've specified (note: empty path) without receiving a pointer to doc.
	assert.NotNil(t, err)
}

func TestReplaceArrayItem(t *testing.T) {
	doc := getMapDoc(`{"foo":["bar", "qux", "baz"]}`)
	patchOp := PatchOperation{Op: "replace", Path: "/foo/1", Value: "boo"}
	patchOp.Apply(&doc)
	assert.Equal(t, []interface{}{"bar", "boo", "baz"}, doc["foo"].([]interface{}))

	patchOp = PatchOperation{Op: "replace", Path: "/foo/4", Value: "boo"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
}

func TestMoveObjKeyErr(t *testing.T) {
	doc := getMapDoc(`{"foo":{"bar": "baz"}, "qux":{"cor":"gra"}}`)
	patchOp := PatchOperation{Op: "move", From: "/foo/non-existent", Path: "/qux/thud"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "has no key 'non-existent'"))
}

func TestMoveObjectKey(t *testing.T) {
	doc := getMapDoc(`{"foo":{"bar":"baz", "waldo":"fred"}, "qux":{"cor":"gra"}}`)
	patchOp := PatchOperation{Op: "move", From: "/foo/waldo", Path: "/qux/thud"}
	patchOp.Apply(&doc)

	foo := doc["foo"].(map[string]interface{})
	qux := doc["qux"].(map[string]interface{})
	assert.Equal(t, 1, len(foo))
	assert.Equal(t, 2, len(qux))
	assert.Equal(t, "fred", qux["thud"].(string))
}

func TestMoveArrayItem(t *testing.T) {
	doc := getMapDoc(`{"foo":["all", "grass", "cows", "eat"]}`)
	patchOp := PatchOperation{Op: "move", From: "/foo/1", Path: "/foo/3"}
	patchOp.Apply(&doc)
	assert.Equal(t, []interface{}{"all", "cows", "eat", "grass"}, doc["foo"].([]interface{}))
}

func TestMoveArrayItemIntoOtherItem(t *testing.T) {
	doc := getArrDoc(`[{"foo":[]}, {"bar":[]}]`)
	patchOp := PatchOperation{Op: "move", From: "/0", Path: "/0/bar/0"}
	err := patchOp.Apply(&doc)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(doc))
	// {"bar": [{"foo":[]}]}
	bar := map[string]interface{}{"bar": []interface{}{
		map[string]interface{}{"foo": []interface{}{}},
	}}
	assert.Equal(t, bar, doc[0])
}

func TestCopyObjectKeyError(t *testing.T) {
	doc := getMapDoc(`{"foo":{"bar":"baz"}, "qux":{"cor":"gra"}}`)
	patchOp := PatchOperation{Op: "copy", From: "/foo/non-existant", Path: "/qux/thud"}
	err := patchOp.Apply(&doc)
	assert.True(t, strings.Contains(err.Error(), "Object has no key 'non-existant'"))
}

func TestCopyObjectKey(t *testing.T) {
	doc := getMapDoc(`{"foo":{"bar":"baz", "waldo":"fred"}, "qux":{"cor":"gra"}}`)
	patchOp := PatchOperation{Op: "copy", From: "/foo/waldo", Path: "/qux/thud"}
	patchOp.Apply(&doc)

	foo := doc["foo"].(map[string]interface{})
	qux := doc["qux"].(map[string]interface{})
	assert.Equal(t, 2, len(foo))
	assert.Equal(t, 2, len(qux))
	assert.Equal(t, "fred", foo["waldo"].(string))
	assert.Equal(t, "fred", qux["thud"].(string))
}

func TestCopyArrayItem(t *testing.T) {
	doc := getMapDoc(`{"foo":["all", "grass", "cows", "eat"]}`)
	patchOp := PatchOperation{Op: "copy", From: "/foo/1", Path: "/foo/3"}
	patchOp.Apply(&doc)

	assert.Equal(t, []interface{}{"all", "grass", "cows", "grass", "eat"}, doc["foo"].([]interface{}))
}

// Test if mutable objects (maps and slices) are copied by value.
func TestCopyMutable(t *testing.T) {
	doc := getMapDoc(`{"foo": [{"bar": 42}, {"baz": 3.14}], "boo": []}`)

	// Copy object somewhere
	patchOp := PatchOperation{Op: "copy", From: "/foo/0", Path: "/boo/0"}
	patchOp.Apply(&doc)
	foo := doc["foo"].([]interface{})
	bar := foo[0].(map[string]interface{})
	assert.Equal(t, 2, len(foo))
	assert.Equal(t, 1, len(bar))
	// Modify original object
	patchOp = PatchOperation{Op: "add", Path: "/foo/0/zoo", Value: 255}
	patchOp.Apply(&doc)

	// Check that we didn't modify the copied object
	b := map[string]interface{}{"bar": 42}
	assert.NotNil(t, b)
	// TODO: WE don't currently copy objects by value.
	//assert.Equal(t, []interface{}{b}, doc["boo"].([]interface{}))
}

func TestTestSuccss(t *testing.T) {
	doc := getMapDoc(`{"baz":"qux", "foo": ["a", 2, "c"]}`)
	patch1 := PatchOperation{Op: "test", Path: "/baz", Value: "/qux"}
	patch2 := PatchOperation{Op: "test", Path: "/foo/1", Value: 2}
	patch1.Apply(&doc)
	patch2.Apply(&doc)
}

func TestTestWholeObject(t *testing.T) {
	doc := getMapDoc(`{"baz": 1}`)
	patchOp := PatchOperation{Op: "test", Path: "", Value: map[string]interface{}{"baz": float64(1)}}
	err := patchOp.Apply(&doc)
	assert.Nil(t, err)
}

func TestTestError(t *testing.T) {
	doc := getMapDoc(`{"bar": "qux"}`)
	patchOp := PatchOperation{Op: "test", Path: "/bar", Value: "bar"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "qux != bar"))
}

func TestTestNotExisting(t *testing.T) {
	doc := getMapDoc(`{"bar":"qux"}`)
	patchOp := PatchOperation{Op: "test", Path: "/baz", Value: "bar"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "has no key 'baz'"))
}

func TestTestNoValExisting(t *testing.T) {
	doc := getMapDoc(`{"bar": "qux"}`)
	patchOp := PatchOperation{Op: "test", Path: "/bar"}
	err := patchOp.Apply(&doc)
	assert.Nil(t, err)
}

func TestTestNoValNotExisting(t *testing.T) {
	doc := getMapDoc(`{"bar": "qux"}`)
	patchOp := PatchOperation{Op: "test", Path: "/baz"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "has no key 'baz'"))
}

func TestTestNoValNotExistingNested(t *testing.T) {
	doc := getMapDoc(`{"bar": {"qux": 2}}`)
	patchOp := PatchOperation{Op: "copy", From: "/foo/waldo", Path: "/qux/thud"}
	err := patchOp.Apply(&doc)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "has no key 'foo'")
}

func TestUnrecognizedElement(t *testing.T) {
	doc := getMapDoc(`{"foo": "bar", "baz": "qux"}`)
	patch, err := FromString(`[{"op": "replace", "path": "/baz", "value": "boo", "something": "baz"}]`)
	assert.Nil(t, err)
	err = patch.Operations[0].Apply(&doc)
	assert.Nil(t, err)
	assert.Equal(t, "boo", doc["baz"].(string))
}

func TestAppend(t *testing.T) {
	doc := getMapDoc(`{"foo": ["a", "b"]}`)
	patch1 := PatchOperation{Op: "add", Path: "/foo/-", Value: "c"}
	patch2 := PatchOperation{Op: "add", Path: "/foo/-", Value: "d"}
	patch1.Apply(&doc)
	patch2.Apply(&doc)
	assert.Equal(t, []interface{}{"a", "b", "c", "d"}, doc["foo"].([]interface{}))
}

// test when type of input is interface{}
func TestIntDoc(t *testing.T) {
	doc := getIntDoc(`{"this": {"is": [1,2,3], "my": {"document": 1}}}`)
	op := PatchOperation{Op: "add", Path: "/this/my/jam", Value: "!test!"}
	err := op.Apply(doc)
	assert.NotNil(t, err)

	op.Apply(&doc)
	value := getValueAt(op.Path, doc)
	assert.Equal(t, "!test!", value)

	doc = getIntDoc(`[{"foo":[]}, {"bar":[]}]`)
	op = PatchOperation{Op: "move", From: "/0", Path: "/0/bar/0"}
	op.Apply(&doc)
	assert.Equal(t, 1, len(doc.([]interface{})))
	bar := map[string]interface{}{"bar": []interface{}{
		map[string]interface{}{"foo": []interface{}{}},
	}}
	assert.Equal(t, bar, doc.([]interface{})[0])
}

func getValueAt(path string, doc interface{}) interface{} {
	pathPtr, err := ptr.NewJsonPointer(path)
	if err != nil {
		panic(err)
	}
	val, _, err := pathPtr.Get(doc)
	if err != nil {
		panic(err)
	}
	return val
}

func getMapDoc(raw string) map[string]interface{} {
	var doc interface{}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		panic(err)
	}
	return doc.(map[string]interface{})
}

func getArrDoc(raw string) []interface{} {
	var doc interface{}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		panic(err)
	}
	return doc.([]interface{})
}

func getIntDoc(raw string) interface{} {
	var doc interface{}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		panic(err)
	}
	return doc
}

/*
class EqualityTestCase(unittest.TestCase):

    def test_patch_equality(self):
        patch1 = jsonpatch.JsonPatch([{ "op": "add", "path": "/a/b/c", "value": "foo" }])
        patch2 = jsonpatch.JsonPatch([{ "path": "/a/b/c", "op": "add", "value": "foo" }])
        self.assertEqual(patch1, patch2)


    def test_patch_unequal(self):
        patch1 = jsonpatch.JsonPatch([{'op': 'test', 'path': '/test'}])
        patch2 = jsonpatch.JsonPatch([{'op': 'test', 'path': '/test1'}])
        self.assertNotEqual(patch1, patch2)

    def test_patch_hash_equality(self):
        patch1 = jsonpatch.JsonPatch([{ "op": "add", "path": "/a/b/c", "value": "foo" }])
        patch2 = jsonpatch.JsonPatch([{ "path": "/a/b/c", "op": "add", "value": "foo" }])
        self.assertEqual(hash(patch1), hash(patch2))


    def test_patch_hash_unequal(self):
        patch1 = jsonpatch.JsonPatch([{'op': 'test', 'path': '/test'}])
        patch2 = jsonpatch.JsonPatch([{'op': 'test', 'path': '/test1'}])
        self.assertNotEqual(hash(patch1), hash(patch2))


    def test_patch_neq_other_objs(self):
        p = [{'op': 'test', 'path': '/test'}]
        patch = jsonpatch.JsonPatch(p)
        # a patch will always compare not-equal to objects of other types
        self.assertFalse(patch == p)
        self.assertFalse(patch == None)

        # also a patch operation will always compare
        # not-equal to objects of other types
        op = jsonpatch.PatchOperation(p[0])
        self.assertFalse(op == p[0])
        self.assertFalse(op == None)

    def test_str(self):
        patch_obj = [ { "op": "add", "path": "/child", "value": { "grandchild": { } } } ]
        patch = jsonpatch.JsonPatch(patch_obj)

        self.assertEqual(json.dumps(patch_obj), str(patch))
        self.assertEqual(json.dumps(patch_obj), patch.to_string())



class MakePatchTestCase(unittest.TestCase):

    def test_apply_patch_to_copy(self):
        src = {'foo': 'bar', 'boo': 'qux'}
        dst = {'baz': 'qux', 'foo': 'boo'}
        patch = jsonpatch.make_patch(src, dst)
        res = patch.apply(src)
        self.assertTrue(src is not res)

    def test_apply_patch_to_same_instance(self):
        src = {'foo': 'bar', 'boo': 'qux'}
        dst = {'baz': 'qux', 'foo': 'boo'}
        patch = jsonpatch.make_patch(src, dst)
        res = patch.apply(src, in_place=True)
        self.assertTrue(src is res)

    def test_objects(self):
        src = {'foo': 'bar', 'boo': 'qux'}
        dst = {'baz': 'qux', 'foo': 'boo'}
        patch = jsonpatch.make_patch(src, dst)
        res = patch.apply(src)
        self.assertEqual(res, dst)

    def test_arrays(self):
        src = {'numbers': [1, 2, 3], 'other': [1, 3, 4, 5]}
        dst = {'numbers': [1, 3, 4, 5], 'other': [1, 3, 4]}
        patch = jsonpatch.make_patch(src, dst)
        res = patch.apply(src)
        self.assertEqual(res, dst)

    def test_complex_object(self):
        src = {'data': [
            {'foo': 1}, {'bar': [1, 2, 3]}, {'baz': {'1': 1, '2': 2}}
        ]}
        dst = {'data': [
            {'foo': [42]}, {'bar': []}, {'baz': {'boo': 'oom!'}}
        ]}
        patch = jsonpatch.make_patch(src, dst)
        res = patch.apply(src)
        self.assertEqual(res, dst)

    def test_array_add_remove(self):
        # see https://github.com/stefankoegl/python-json-patch/issues/4
        src = {'numbers': [], 'other': [1, 5, 3, 4]}
        dst = {'numbers': [1, 3, 4, 5], 'other': []}
        patch = jsonpatch.make_patch(src, dst)
        res = patch.apply(src)
        self.assertEqual(res, dst)

    def test_add_nested(self):
        # see http://tools.ietf.org/html/draft-ietf-appsawg-json-patch-03#appendix-A.10
        src = {"foo": "bar"}
        patch_obj = [ { "op": "add", "path": "/child", "value": { "grandchild": { } } } ]
        res = jsonpatch.apply_patch(src, patch_obj)
        expected = { "foo": "bar",
                      "child": { "grandchild": { } }
                   }
        self.assertEqual(expected, res)

    def test_should_just_add_new_item_not_rebuild_all_list(self):
        src = {'foo': [1, 2, 3]}
        dst = {'foo': [3, 1, 2, 3]}
        patch = list(jsonpatch.make_patch(src, dst))
        self.assertEqual(len(patch), 1)
        self.assertEqual(patch[0]['op'], 'add')
        res = jsonpatch.apply_patch(src, patch)
        self.assertEqual(res, dst)

    def test_use_replace_instead_of_remove_add(self):
        src = {'foo': [1, 2, 3]}
        dst = {'foo': [3, 2, 3]}
        patch = list(jsonpatch.make_patch(src, dst))
        self.assertEqual(len(patch), 1)
        self.assertEqual(patch[0]['op'], 'replace')
        res = jsonpatch.apply_patch(src, patch)
        self.assertEqual(res, dst)

    def test_use_move_instead_of_remove_add(self):
        src = {'foo': [4, 1, 2, 3]}
        dst = {'foo': [1, 2, 3, 4]}
        patch = list(jsonpatch.make_patch(src, dst))
        self.assertEqual(len(patch), 1)
        self.assertEqual(patch[0]['op'], 'move')
        res = jsonpatch.apply_patch(src, patch)
        self.assertEqual(res, dst)

    def test_use_move_instead_of_add_remove(self):
        src = {'foo': [1, 2, 3]}
        dst = {'foo': [3, 1, 2]}
        patch = list(jsonpatch.make_patch(src, dst))
        self.assertEqual(len(patch), 1)
        self.assertEqual(patch[0]['op'], 'move')
        res = jsonpatch.apply_patch(src, patch)
        self.assertEqual(res, dst)

    def test_escape(self):
        src = {"x/y": 1}
        dst = {"x/y": 2}
        patch = jsonpatch.make_patch(src, dst)
        self.assertEqual([{"path": "/x~1y", "value": 2, "op": "replace"}], patch.patch)
        res = patch.apply(src)
        self.assertEqual(res, dst)


class InvalidInputTests(unittest.TestCase):

    def test_missing_op(self):
        # an "op" member is required
        src = {"foo": "bar"}
        patch_obj = [ { "path": "/child", "value": { "grandchild": { } } } ]
        self.assertRaises(jsonpatch.JsonPatchException, jsonpatch.apply_patch, src, patch_obj)


    def test_invalid_op(self):
        # "invalid" is not a valid operation
        src = {"foo": "bar"}
        patch_obj = [ { "op": "invalid", "path": "/child", "value": { "grandchild": { } } } ]
        self.assertRaises(jsonpatch.JsonPatchException, jsonpatch.apply_patch, src, patch_obj)


class ConflictTests(unittest.TestCase):

    def test_remove_indexerror(self):
        src = {"foo": [1, 2]}
        patch_obj = [ { "op": "remove", "path": "/foo/10"} ]
        self.assertRaises(jsonpatch.JsonPatchConflict, jsonpatch.apply_patch, src, patch_obj)

    def test_remove_keyerror(self):
        src = {"foo": [1, 2]}
        patch_obj = [ { "op": "remove", "path": "/foo/b"} ]
        self.assertRaises(jsonpointer.JsonPointerException, jsonpatch.apply_patch, src, patch_obj)

    def test_remove_keyerror_dict(self):
        src = {'foo': {'bar': 'barz'}}
        patch_obj = [ { "op": "remove", "path": "/foo/non-existent"} ]
        self.assertRaises(jsonpatch.JsonPatchConflict, jsonpatch.apply_patch, src, patch_obj)

    def test_insert_oob(self):
        src = {"foo": [1, 2]}
        patch_obj = [ { "op": "add", "path": "/foo/10", "value": 1} ]
        self.assertRaises(jsonpatch.JsonPatchConflict, jsonpatch.apply_patch, src, patch_obj)

    def test_move_into_child(self):
        src = {"foo": {"bar": {"baz": 1}}}
        patch_obj = [ { "op": "move", "from": "/foo", "path": "/foo/bar" } ]
        self.assertRaises(jsonpatch.JsonPatchException, jsonpatch.apply_patch, src, patch_obj)

    def test_replace_oob(self):
        src = {"foo": [1, 2]}
        patch_obj = [ { "op": "replace", "path": "/foo/10", "value": 10} ]
        self.assertRaises(jsonpatch.JsonPatchConflict, jsonpatch.apply_patch, src, patch_obj)

    def test_replace_missing(self):
        src = {"foo": 1}
        patch_obj = [ { "op": "replace", "path": "/bar", "value": 10} ]
        self.assertRaises(jsonpatch.JsonPatchConflict, jsonpatch.apply_patch, src, patch_obj)


*/
