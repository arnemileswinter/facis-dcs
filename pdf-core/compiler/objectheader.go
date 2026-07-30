package compiler

import (
	"bytes"
	"strconv"
)

// Locating an indirect object by its id, per ISO 32000-1 7.3.10 and 7.3.8.
//
// A definition is "<id> <gen> obj", the tokens separated by ANY run of white
// space (7.2.3), and it ends at the "endobj" keyword. Every lookup here resolves
// the definition a reader resolves and reads only bytes belonging to it, because
// each of the three ways to get that wrong hands a checker bytes the rendered
// document does not contain:
//
//   - A bare substring match for "19 0 obj" also hits inside "100019 0 obj", so
//     a decoy object whose id merely ENDS in the wanted digits supplies the
//     bytes. A header therefore only counts when its id starts a line.
//   - A fixed "%d 0 obj" spelling misses "19  0 obj" and the superseding
//     "19 1 obj" — a freed and reused object comes back with a raised generation
//     — and silently falls back to an EARLIER definition.
//   - A forward scan for the "stream" keyword that is not clipped at the
//     object's own "endobj" runs into the NEXT object and returns its stream.
//     "stream\n" alone does it: 7.3.8.1 permits CRLF after the keyword and most
//     producers emit it, so the object's own stream is stepped over.

var (
	objKeyword       = []byte("obj")
	endobjKeyword    = []byte("endobj")
	streamKeyword    = []byte("stream")
	endstreamKeyword = []byte("endstream")
)

// isPDFWhitespace reports whether b is one of the six PDF white-space
// characters (ISO 32000-1 table 1).
func isPDFWhitespace(b byte) bool {
	switch b {
	case 0x00, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

// isPDFDelimiter reports whether b is one of the delimiter characters
// (ISO 32000-1 table 2), which terminate a keyword just as white space does.
func isPDFDelimiter(b byte) bool {
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

// objectHeader is one definition of an indirect object.
type objectHeader struct {
	// start is the offset of the object id's first digit.
	start int
	// body is the offset at which the definition's content begins: past the
	// "obj" keyword and the end-of-line closing it.
	body int
	// end is the offset of the "endobj" keyword closing this definition, or
	// the end of the file when it has none.
	end int
}

// objectHeaders returns every definition of objID in pdf, in file order.
func objectHeaders(pdf []byte, objID int) []objectHeader {
	id := []byte(strconv.Itoa(objID))
	var headers []objectHeader
	for searchFrom := 0; searchFrom+len(id) <= len(pdf); {
		rel := bytes.Index(pdf[searchFrom:], id)
		if rel < 0 {
			break
		}
		at := searchFrom + rel
		searchFrom = at + 1
		if at != 0 && pdf[at-1] != '\n' && pdf[at-1] != '\r' {
			continue
		}
		body, ok := objectHeaderBody(pdf, at+len(id))
		if !ok {
			continue
		}
		headers = append(headers, objectHeader{start: at, body: body, end: objectDefinitionEnd(pdf, body)})
	}
	return headers
}

// objectHeaderBody parses the rest of a header — white space, the generation
// number, white space, "obj" — starting just past the object id, and returns
// the offset at which the definition's content begins.
func objectHeaderBody(pdf []byte, pos int) (int, bool) {
	pos, ok := skipPDFWhitespace(pdf, pos)
	if !ok {
		return 0, false
	}
	generation := pos
	for pos < len(pdf) && pdf[pos] >= '0' && pdf[pos] <= '9' {
		pos++
	}
	if pos == generation {
		return 0, false
	}
	pos, ok = skipPDFWhitespace(pdf, pos)
	if !ok {
		return 0, false
	}
	if !bytes.HasPrefix(pdf[pos:], objKeyword) {
		return 0, false
	}
	pos += len(objKeyword)
	if pos < len(pdf) && !isPDFWhitespace(pdf[pos]) && !isPDFDelimiter(pdf[pos]) {
		return 0, false
	}
	return pos + endOfLineLength(pdf, pos), true
}

// skipPDFWhitespace advances past a run of at least one white-space character.
func skipPDFWhitespace(pdf []byte, pos int) (int, bool) {
	start := pos
	for pos < len(pdf) && isPDFWhitespace(pdf[pos]) {
		pos++
	}
	return pos, pos > start
}

// endOfLineLength returns the length of the end-of-line sequence at pos — CRLF,
// LF or a bare CR — or 0 when there is none.
func endOfLineLength(pdf []byte, pos int) int {
	if pos >= len(pdf) {
		return 0
	}
	if pdf[pos] == '\n' {
		return 1
	}
	if pdf[pos] == '\r' {
		if pos+1 < len(pdf) && pdf[pos+1] == '\n' {
			return 2
		}
		return 1
	}
	return 0
}

// objectDefinitionEnd returns the offset of the "endobj" keyword closing the
// definition whose content starts at body, or the end of the file when the
// definition is unterminated. Every scan inside an object is clipped here.
func objectDefinitionEnd(pdf []byte, body int) int {
	for at := body; at < len(pdf); {
		rel := bytes.Index(pdf[at:], endobjKeyword)
		if rel < 0 {
			break
		}
		found := at + rel
		at = found + 1
		if found > 0 && !isPDFWhitespace(pdf[found-1]) && !isPDFDelimiter(pdf[found-1]) {
			continue // a word merely ending in "endobj"
		}
		return found
	}
	return len(pdf)
}

// firstObjectHeader returns the FIRST definition of objID — the genesis object
// of an incrementally updated document.
func firstObjectHeader(pdf []byte, objID int) (objectHeader, bool) {
	headers := objectHeaders(pdf, objID)
	if len(headers) == 0 {
		return objectHeader{}, false
	}
	return headers[0], true
}

// lastObjectHeader returns the definition of objID a reader resolves — the
// last one, since an incremental update supersedes an object by appending a new
// definition.
func lastObjectHeader(pdf []byte, objID int) (objectHeader, bool) {
	headers := objectHeaders(pdf, objID)
	if len(headers) == 0 {
		return objectHeader{}, false
	}
	return headers[len(headers)-1], true
}

// objectStreamData returns the half-open range of a definition's raw stream
// data: past the "stream" keyword and the end-of-line after it, up to the
// end-of-line preceding "endstream". Both scans stay inside the definition.
func objectStreamData(pdf []byte, header objectHeader) (start, end int, ok bool) {
	start, ok = streamDataOffset(pdf, header)
	if !ok {
		return 0, 0, false
	}
	rel := bytes.Index(pdf[start:header.end], endstreamKeyword)
	if rel < 0 {
		return 0, 0, false
	}
	end = start + rel
	if end > start && pdf[end-1] == '\n' {
		end--
	}
	if end > start && pdf[end-1] == '\r' {
		end--
	}
	return start, end, true
}

// streamDataOffset locates the definition's "stream" keyword and returns the
// offset just past the CRLF or LF that ISO 32000-1 7.3.8.1 requires after it.
func streamDataOffset(pdf []byte, header objectHeader) (int, bool) {
	for at := header.body; at < header.end; {
		rel := bytes.Index(pdf[at:header.end], streamKeyword)
		if rel < 0 {
			return 0, false
		}
		found := at + rel
		at = found + 1
		if found > 0 && !isPDFWhitespace(pdf[found-1]) && !isPDFDelimiter(pdf[found-1]) {
			continue // the tail of "endstream", or a name ending in "stream"
		}
		data := found + len(streamKeyword)
		switch {
		case data < len(pdf) && pdf[data] == '\n':
			return data + 1, true
		case data+1 < len(pdf) && pdf[data] == '\r' && pdf[data+1] == '\n':
			return data + 2, true
		}
	}
	return 0, false
}

// lastObjectStreamData returns the raw stream data of the definition of objID a
// reader resolves.
func lastObjectStreamData(pdf []byte, objID int) (start, end int, ok bool) {
	header, found := lastObjectHeader(pdf, objID)
	if !found {
		return 0, 0, false
	}
	return objectStreamData(pdf, header)
}

// firstObjectStreamData returns the raw stream data of objID's genesis
// definition.
func firstObjectStreamData(pdf []byte, objID int) (start, end int, ok bool) {
	header, found := firstObjectHeader(pdf, objID)
	if !found {
		return 0, 0, false
	}
	return objectStreamData(pdf, header)
}

// lastObjectBody returns the range of the definition of objID a reader
// resolves — its dictionary, without the end-of-line closing it before
// "endobj".
func lastObjectBody(pdf []byte, objID int) (start, end int, ok bool) {
	header, found := lastObjectHeader(pdf, objID)
	if !found {
		return 0, 0, false
	}
	end = header.end
	if end > header.body && pdf[end-1] == '\n' {
		end--
	}
	if end > header.body && pdf[end-1] == '\r' {
		end--
	}
	return header.body, end, true
}
