package jsonpatch_test

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/cameront/go-jsonpatch"
)

func Example() {
	patch := jsonpatch.Patch{
		Operations: []jsonpatch.PatchOperation{
			{Op: jsonpatch.Add, Path: "/foo", Value: "bar"},
			{Op: jsonpatch.Add, Path: "/baz", Value: []interface{}{1, 2, 3}},
			{Op: jsonpatch.Remove, Path: "/baz/1"},
			{Op: jsonpatch.Test, Path: "/baz", Value: []interface{}{1, 3}},
			{Op: jsonpatch.Replace, Path: "/baz/0", Value: 42},
			{Op: jsonpatch.Remove, Path: "/baz/1"},
		},
	}

	// apply the patch to an empty document
	doc := make(map[string]interface{})
	err := patch.Apply(&doc)
	if err != nil {
		log.Fatalf("apply: %v", err)
	}

	fmt.Println(doc["foo"])
	fmt.Println(doc["baz"])

	// Output:
	// bar
	// [42]
}

func ExampleMakePatch() {
	var src map[string]interface{}
	rawsrc := `{"foo":"bar","numbers":[1,3,4,8]}`
	err := json.Unmarshal([]byte(rawsrc), &src)
	if err != nil {
		panic(err)
	}

	var dst map[string]interface{}
	rawdst := `{"foo":"qux","numbers":[1,4,7]}`
	err = json.Unmarshal([]byte(rawdst), &dst)
	if err != nil {
		panic(err)
	}

	patch, err := jsonpatch.MakePatch(src, dst)
	if err != nil {
		panic(err)
	}
	err = patch.Apply(&src)
	if err != nil {
		panic(err)
	}

	fmt.Println(src["foo"])
	fmt.Println(src["numbers"])

	// Output:
	// qux
	// [1 4 7]
}
