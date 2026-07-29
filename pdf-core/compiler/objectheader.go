package compiler

import (
	"bytes"
	"fmt"
)

// findFirstObjectHeaderOffset is findLastObjectHeaderOffset's counterpart for
// the first definition of an object, anchored on a line start so an id that is
// a digit-suffix of another ("19" within "100019") cannot be mistaken for it.
func findFirstObjectHeaderOffset(pdf []byte, objID int) int {
	headerAtStart := []byte(fmt.Sprintf("%d 0 obj\n", objID))
	if bytes.HasPrefix(pdf, headerAtStart) {
		return 0
	}
	rel := bytes.Index(pdf, []byte(fmt.Sprintf("\n%d 0 obj\n", objID)))
	if rel < 0 {
		return -1
	}
	return rel + 1
}
