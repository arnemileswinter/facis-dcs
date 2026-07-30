package compiler

import (
	"bytes"
	"fmt"
)

// A PDF object header is "<id> 0 obj". Matched as a bare substring it also hits
// inside "100019 0 obj", so a decoy object whose id merely ENDS in the wanted
// digits can supply the bytes a checker reads while a conforming reader follows
// the xref to a different object. A header therefore only counts here when its
// id starts a line (or the file) and the "obj" keyword is closed by PDF white
// space — which includes the CRLF end-of-line every reader accepts, and which
// an "\n%d 0 obj\n" match would miss, silently falling back to an EARLIER
// definition: the same evasion in a different spelling.

// isPDFWhitespace reports whether b is one of the six PDF white-space
// characters (ISO 32000-1 table 1).
func isPDFWhitespace(b byte) bool {
	switch b {
	case 0x00, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

// objectHeaderOffsets returns the offset of every definition of objID's header
// in pdf, in file order.
func objectHeaderOffsets(pdf []byte, objID int) []int {
	marker := []byte(fmt.Sprintf("%d 0 obj", objID))
	var offsets []int
	for searchFrom := 0; searchFrom+len(marker) <= len(pdf); {
		rel := bytes.Index(pdf[searchFrom:], marker)
		if rel < 0 {
			break
		}
		at := searchFrom + rel
		end := at + len(marker)
		startsLine := at == 0 || pdf[at-1] == '\n' || pdf[at-1] == '\r'
		if startsLine && end < len(pdf) && isPDFWhitespace(pdf[end]) {
			offsets = append(offsets, at)
		}
		searchFrom = at + 1
	}
	return offsets
}

// findFirstObjectHeaderOffset returns the offset of the FIRST definition of
// objID — the genesis object of an incrementally updated document — or -1.
func findFirstObjectHeaderOffset(pdf []byte, objID int) int {
	offsets := objectHeaderOffsets(pdf, objID)
	if len(offsets) == 0 {
		return -1
	}
	return offsets[0]
}

// findLastObjectHeaderOffset returns the offset of the LAST definition of
// objID — the one a reader resolves, since an incremental update supersedes an
// object by appending a new definition — or -1.
func findLastObjectHeaderOffset(pdf []byte, objID int) int {
	offsets := objectHeaderOffsets(pdf, objID)
	if len(offsets) == 0 {
		return -1
	}
	return offsets[len(offsets)-1]
}

// lastObjectBodyOffset returns the offset at which the body of objID's current
// definition begins: past the header and past the end-of-line closing it,
// whether that is "\n" or "\r\n".
func lastObjectBodyOffset(pdf []byte, objID int) (int, bool) {
	header := findLastObjectHeaderOffset(pdf, objID)
	if header < 0 {
		return 0, false
	}
	body := header + len(fmt.Sprintf("%d 0 obj", objID))
	if body < len(pdf) && pdf[body] == '\r' {
		body++
	}
	if body < len(pdf) && pdf[body] == '\n' {
		body++
	}
	return body, true
}
