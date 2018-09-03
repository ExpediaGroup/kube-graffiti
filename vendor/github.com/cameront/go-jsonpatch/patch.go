/*
Ported from github.com/stefankoegl/python-json-patch
*/
package jsonpatch

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Patch is a list of PatchOperations.
type Patch struct {
	Operations []PatchOperation
}

func (p Patch) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Operations)
}

func (p *Patch) UnmarshalJSON(b []byte) error {
	ops := []PatchOperation{}
	err := json.Unmarshal(b, &ops)
	if err != nil {
		return err
	}
	*p = Patch{ops}
	return nil
}

func (p *Patch) Apply(doc interface{}) (err error) {
	for _, op := range p.Operations {
		err = op.Apply(doc)
		if err != nil {
			return err
		}
	}
	return nil
}

func FromString(str string) (Patch, error) {
	patch := Patch{}
	err := json.Unmarshal([]byte(str), &patch)
	return patch, err
}

// MakePatch generates a patch by comparing two documents.
func MakePatch(src interface{}, dst interface{}) (Patch, error) {
	return MakeDiff(src, dst)
}

func MakeDiff(src, dst interface{}) (Patch, error) {
	mapSrc, ok := src.(map[string]interface{})
	if !ok {
		return Patch{}, fmt.Errorf("not a valid map: %T", src)
	}
	mapDst, ok := dst.(map[string]interface{})
	if !ok {
		return Patch{}, fmt.Errorf("not a valid map: %T", dst)
	}

	patch := Patch{[]PatchOperation{}}
	for _, opPtr := range compareDicts("", mapSrc, mapDst) {
		patch.Operations = append(patch.Operations, *opPtr)
	}
	return patch, nil
}

func compareValues(path string, value, other interface{}) []*PatchOperation {
	operations := []*PatchOperation{}
	if reflect.DeepEqual(value, other) {
		return operations
	}
	valueKind := reflect.ValueOf(value).Kind()
	otherKind := reflect.ValueOf(value).Kind()
	if valueKind == reflect.Map && otherKind == reflect.Map {
		mapValue := value.(map[string]interface{})
		mapOther := other.(map[string]interface{})
		operations = append(operations, compareDicts(path, mapValue, mapOther)...)
	} else if (valueKind == reflect.Slice) && (otherKind == reflect.Slice) {
		slValue := value.([]interface{})
		slOther := other.([]interface{})
		operations = append(operations, compareLists(path, slValue, slOther)...)
	} else {
		replace := &PatchOperation{Op: "replace", Path: path, Value: other}
		operations = append(operations, replace)
	}
	return operations
}

func compareDicts(path string, src, dst map[string]interface{}) []*PatchOperation {
	operations := []*PatchOperation{}
	for key, _ := range src {
		currentPath := path + "/" + key
		if _, ok := dst[key]; !ok {
			remove := &PatchOperation{Op: "remove", Path: currentPath}
			operations = append(operations, remove)
			continue
		}
		for _, operation := range compareValues(currentPath, src[key], dst[key]) {
			operations = append(operations, operation)
		}
	}
	for key, _ := range dst {
		currentPath := path + "/" + key
		if _, ok := src[key]; !ok {
			add := &PatchOperation{Op: "add", Path: currentPath, Value: dst[key]}
			operations = append(operations, add)
		}
	}
	return operations
}

func compareLists(path string, src, dst []interface{}) []*PatchOperation {
	return optimize(compare(path, src, dst, splitByCommonSeq(src, dst, &intPair{0, -1}, &intPair{0, -1})))
}

func longestCommonSubsequence(src, dst []interface{}) {
	panic("lcs")
}

// Returns pair of ranges of longest common subsequence for the `src`
// and `dst` lists.
//
//		>>> src = [1, 2, 3, 4]
//		>>> dst = [0, 1, 2, 3, 5]
//		>>> # The longest common subsequence for these lists is [1, 2, 3]
//		... # which is located at (0, 3) index range for src list and (1, 4) for
//		... # dst one. Tuple of these ranges we should get back.
//		... assert ((0, 3), (1, 4)) == _longest_common_subseq(src, dst)
func longestCommonSubseq(src, dst []interface{}) (rangeSrc *intPair, rangeDst *intPair) {
	lenSrc, lenDst := len(src), len(dst)
	dRange := []int{}
	for i := 0; i < lenDst; i++ {
		dRange = append(dRange, i)
	}
	matrix := [][]int{}
	//matrix = [[0] * ldst for _ in range(lsrc)]
	for i := 0; i < lenSrc; i++ {
		row := []int{}
		for j := 0; j < lenDst; j++ {
			row = append(row, 0)
		}
		matrix = append(matrix, row)
	}
	z := 0 // length of the longest subsequence
	rangeSrc, rangeDst = nil, nil
	for i := 0; i < lenSrc; i++ {
		for di := 0; di < len(dRange); di++ {
			j := dRange[di]
			if src[i] == dst[j] {
				if i == 0 || j == 0 {
					matrix[i][j] = 1
				} else {
					matrix[i][j] = matrix[i-1][j-1] + 1
				}
				if matrix[i][j] > z {
					z = matrix[i][j]
				}
				if matrix[i][j] == z {
					rangeSrc = &intPair{i - z + 1, i + 1}
					rangeDst = &intPair{j - z + 1, j + 1}
				}
			} else {
				matrix[i][j] = 0
			}
		}
	}
	return rangeSrc, rangeDst
}

type commonSeqNode struct {
	left     *intPair
	leftPtr  *commonSeqNode
	right    *intPair
	rightPtr *commonSeqNode
}

// Recursively splits the `dst` list onto two parts: left and right.
// The left part contains differences on left from common subsequence,
// same as the right part by for other side.
//
// To easily understand the process let's take two lists: [0, 1, 2, 3] as
// `src` and [1, 2, 4, 5] for `dst`. If we've tried to generate the binary tree
// where nodes are common subsequence for both lists, leaves on the left
// side are subsequence for `src` list and leaves on the right one for `dst`,
// our tree would looks like::
//
//		    [1, 2]
//		   /     \
//		[0]       []
//		         /  \
//		      [3]   [4, 5]
//
// This function generate the similar structure as flat tree, but without
// nodes with common subsequences - since we're don't need them - only with
// left and right leaves::
//
//		    []
//		   / \
//		[0]  []
//		    / \
//		 [3]  [4, 5]
//
// The `bx` is the absolute range for currently processed subsequence of `src`
// list.  The `by` means the same, but for the `dst` list.
func splitByCommonSeq(src, dst []interface{}, bx, by *intPair) commonSeqNode {
	// Prevent useless comparisons in future
	if bx.a == bx.b {
		bx = nil
	}
	if by.a == by.b {
		by = nil
	}

	if len(src) == 0 {
		return commonSeqNode{nil, nil, by, nil}
	} else if len(dst) == 0 {
		return commonSeqNode{bx, nil, nil, nil}
	}

	// note that these ranges are relative for processed sublists
	x, y := longestCommonSubseq(src, dst)
	if x == nil || y == nil {
		// no more any common subsequence
		return commonSeqNode{bx, nil, by, nil}
	}

	retA := splitByCommonSeq(
		src[:x.a], dst[:y.a], &intPair{bx.a, bx.a + x.a}, &intPair{by.a, by.a + y.a})
	retB := splitByCommonSeq(
		src[x.b:], dst[y.b:], &intPair{bx.a + x.b, bx.a + len(src)}, &intPair{bx.a + y.b, bx.a + len(dst)})
	return commonSeqNode{nil, &retA, nil, &retB}
}

// Same as :func:`_compare_with_shift` but strips emitted `shift` value.
func compare(path string, src, dst []interface{}, seqNode commonSeqNode) []*PatchOperation {
	patchOps := []*PatchOperation{}
	zero := 0
	if seqNode.leftPtr != nil || seqNode.rightPtr != nil {
		for _, indexedOp := range compareWithShift(path, src, dst, seqNode.leftPtr, seqNode.rightPtr, &zero) {
			patchOps = append(patchOps, indexedOp.patchOperation)
		}
	} else if seqNode.left != nil || seqNode.right != nil {
		for _, indexedOp := range compareWithShift(path, src, dst, seqNode.left, seqNode.right, &zero) {
			patchOps = append(patchOps, indexedOp.patchOperation)
		}
	}
	return patchOps
}

// Recursively compares differences from `left` and `right` sides
// from common subsequences.
//
// The `shift` parameter is used to store index shift which caused
// by ``add`` and ``remove`` operations.
//
// Yields JSON patch operations and list index shift.
func compareWithShift(path string, src, dst []interface{}, left, right interface{}, shift *int) []indexedOp {
	result := []indexedOp{}
	switch t := left.(type) {
	case *commonSeqNode:
		if t != nil {
			// left points to EITHER ptrs or values
			if t.leftPtr != nil || t.rightPtr != nil {
				result = append(result, compareWithShift(path, src, dst, t.leftPtr, t.rightPtr, shift)...)
			} else {
				result = append(result, compareWithShift(path, src, dst, t.left, t.right, shift)...)
			}
		}
	case *intPair:
		if t != nil {
			result = append(result, compareLeft(path, src, t.a, t.b, shift)...)
		}
	}

	switch t := right.(type) {
	case *commonSeqNode:
		if t != nil {
			// right points to EITHER ptrs or values
			if t.leftPtr != nil || t.rightPtr != nil {
				result = append(result, compareWithShift(path, src, dst, t.leftPtr, t.rightPtr, shift)...)
			} else {
				result = append(result, compareWithShift(path, src, dst, t.left, t.right, shift)...)
			}
		}
	case *intPair:
		if t != nil {
			result = append(result, compareRight(path, dst, t.a, t.b, shift)...)
		}
	}
	return result
}

type indexedOp struct {
	patchOperation *PatchOperation
	shift          int
}

type intPair struct {
	a int
	b int
}

// Yields JSON patch ``remove`` operations for elements that are only
// exists in the `src` list.
func compareLeft(path string, src []interface{}, leftStart, leftEnd int, shift *int) []indexedOp {
	result := []indexedOp{}
	if leftEnd == -1 {
		leftEnd = len(src)
	}
	//# we need to `remove` elements from list tail to not deal with index shift
	start := leftEnd + *shift - 1
	end := leftStart + *shift
	for i := start; i >= end; i-- {
		indexPath := path + "/" + strconv.Itoa(i)
		// yes, there should be any value field, but we'll use it
		// to apply `move` optimization a bit later and will remove
		// it in _optimize function.
		idxOp := indexedOp{
			patchOperation: &PatchOperation{Op: "remove", Value: src[i-*shift], Path: indexPath},
			shift:          *shift - 1,
		}
		result = append(result, idxOp)
		*shift--
	}
	return result
}

// Yields JSON patch ``add`` operations for elements that are only
// exists in the `dst` list
func compareRight(path string, dst []interface{}, rightStart, rightEnd int, shift *int) []indexedOp {
	result := []indexedOp{}
	if rightEnd == -1 {
		rightEnd = len(dst)
	}
	for i := rightStart; i < rightEnd; i++ {
		indexPath := path + "/" + strconv.Itoa(i)
		idxOp := indexedOp{
			patchOperation: &PatchOperation{Op: "add", Path: indexPath, Value: dst[i]},
			shift:          *shift + 1,
		}
		result = append(result, idxOp)
		*shift++
	}
	return result
}

// Optimizes operations which was produced by lists comparison.
//     Actually it does two kinds of optimizations:
//     1. Seeks pair of ``remove`` and ``add`` operations against the same path
//        and replaces them with ``replace`` operation.
//     2. Seeks pair of ``remove`` and ``add`` operations for the same value
//        and replaces them with ``move`` operation.
func optimize(operations []*PatchOperation) []*PatchOperation {
	result := []*PatchOperation{}
	opsByPath := map[string]*PatchOperation{}
	opsByValue := map[interface{}]*PatchOperation{}
	for _, op := range operations {
		// could we apply "move" optimization for dict values?
		valueKind := reflect.ValueOf(op.Value).Kind()
		if val, ok := opsByPath[op.Path]; ok {
			optimizeUsingReplace(val, op)
			continue
		}
		hashable := valueKind != reflect.Map && valueKind != reflect.Slice
		if hashable {
			prevItem, inMap := opsByValue[op.Value]
			if inMap {
				// ensure that we processing pair of add-remove ops
				if op.Op == "add" && prevItem.Op == "remove" {
					optimizeUsingMove(prevItem, op)
					delete(opsByValue, op.Value)
					continue
				}
			}
		}
		result = append(result, op)
		opsByPath[op.Path] = op
		if hashable {
			opsByValue[op.Value] = op
		}
	}

	// # cleanup
	//ops_by_path.clear()
	//ops_by_value.clear()
	for _, op := range result {
		if op.Op == "remove" {
			op.Value = nil
		}
	}
	return result
}

// Optimises JSON patch by using ``replace`` operation instead of
// ``remove`` and ``add`` against the same path.
func optimizeUsingReplace(prev, cur *PatchOperation) {
	prev.Op = "replace"
	if cur.Op == "add" {
		prev.Value = cur.Value
	}
}

//Optimises JSON patch by using ``move`` operation instead of
//``remove` and ``add`` against the different paths but for the same value.
func optimizeUsingMove(prevItem, item *PatchOperation) {
	prevItem.Op = "move"
	moveFrom, moveTo := item.Path, prevItem.Path
	if item.Op == "add" {
		moveFrom, moveTo = prevItem.Path, item.Path
	}
	if item.Op == "add" { // first was remove then add
		prevItem.From = moveFrom
		prevItem.Path = moveTo
	} else { // first was add then remove
		fromSplit := strings.Split(moveFrom, "/")
		head := strings.Join(fromSplit[0:len(fromSplit)-1], "/")
		moveFrom := fromSplit[len(fromSplit)-1]
		// since add operation was first it incremented
		// overall index shift value. we have to fix this
		moveFromInt, err := strconv.Atoi(moveFrom)
		if err != nil {
			fmt.Println(err)
		}
		prevItem.From = head + "/" + strconv.Itoa(moveFromInt-1)
		prevItem.Path = moveTo
	}
}
