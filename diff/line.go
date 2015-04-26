package diff

import (
	"bytes"

	"github.com/jamessynge/diffmerge/dm"
)

type LineNo int32
type FileOffset int32
type HashType uint32

type Line interface {
	LineNumber() LineNo // Line 0 and line N+1 (for an N line file) are sentinals
									    // of length 0.
	IsSentinal() bool  // Is this a sentinal (only the first and last)?
	IsStartSentinal() bool  // Is this the start-of-file sentinal?

	// Where does the line appear in the file (in terms of byte position, starting
	// at 0).
	Start() FileOffset
	Length() FileOffset // Not really a file "offset" but oh well.
	End() FileOffset // Start + Length
	Hash() HashType  // Hash of the full line, including leading whitespace,
								   // trailing whitespace and CR/LF (or whatever ends the line).

	// Access to the underlying file buffer. Do not modify the bytes.
	Bytes() []byte	

	// Lines often have leading whitespace, and sometimes trailing (or at least
	// a newline). These methods provide access to the lines without the leading
	// and trailing whitespace.
	ContentStart() FileOffset
	ContentLength() FileOffset
	ContentHash() HashType

	// Access to the underlying file buffer. Do not modify the bytes.
	ContentBytes() []byte

	// At the start of the line, how many leading tabs are there? We presume that
	// leading whitespace consists of zero or more tabs, followed by zero or more
	// spaces, followed by content, or the end of the line (e.g. LF). We cap the
	// count at 255 because the focus of this program is comparing real source
	// code, not arbitrary files.
	LeadingTabsCount() uint8

	// After the leading tabs, how many leading spaces are there?
	LeadingSpacesCount() uint8

	// Count of the number of times that the ContentHash appears in the file.
	// Maximum is 255, but that is OK for rare-ness checking.
	CountInFile() uint8

	// Is this a common, well known line (e.g. "/*" or "#", or an empty line),
	// that probably appears in many/most source files of the same language. Such
	// lines aren't very useful for aligning two files.
	IsWellKnownContent()

	// Does this line contain rare content, i.e. is not well known content, such
	// as "}", and only appears once in this file, or a small number of times if
	// this is a large file (e.g. not more than 1 in 500 lines).
	IsRareLine()

	// Calls FullEquals if fullEquals==true, else calls ContentEquals.
	Equals(other Line, fullEquals bool) bool

	// Is this line exactly equal to the other line?
	FullEquals(other Line) bool

	// Is ContentBytes() of this line equal to the content bytes of the other line?
	ContentEquals(other Line) bool
}

type LineType uint8

const (
	startOfFile LineType = iota
	endOfFile
	normalLine
	wellKnownLine
)

type lineInstance struct {
	file File
	lineNumber LineNo
	start, end, contentStart, contentLength FileOffset
	hash, contentHash HashType
	leadingTabs, leadingSpaces, countInFile uint8
	lineType LineType
}

var theLineHasher = dm.GetLineHasher()

func makeLineInstance(file File, start FileOffset, lineNumber LineNo,
	lineBytes []byte) *lineInstance {
	length := len(lineBytes)
	// There are around 20 space characters in Unicode, but we're only handling
	// ASCII tab and space characters here.
	n := 0
	for ; n < length; n++ {
		if lineBytes[n] != '\t' { break }
	}
	tabCount := n
	for ; n < length; n++ {
		if lineBytes[n] != ' ' { break }
	}
	spaceCount := n - tabCount
	// Content starts after leading whitespace, any mixture of spaces and tabs.
	for ; n < length; n++ {
		b := lineBytes[n]
		if b != ' ' && b != '\t' { break }
	}
	contentIndex := n
	if contentIndex > tabCount + spaceCount {
		// Line doesn't start with "well-formed" indentation, so we can't as readily
		// compare indentations. So, mark this line such that we can detect this.
		tabCount = 255
		spaceCount = 255
	}
	// Search from the end for the first non-whitespace character.
	n = length - 1
TrailingLoop:
	for ; n >= contentIndex ; n-- {
		switch lineBytes[n] {
		case ' ', '\n', '\r', '\t', '\f', '\v':
			// Whitespace.
			continue
		default:
			break TrailingLoop
		}
	}
	contendEndIndex := dm.MaxInt(contentIndex, n) + 1
	contentBytes := lineBytes[contentIndex : contendEndIndex]
	contentLength := len(contentBytes)
	lineHash, contentHash := theLineHasher.Compute2(lineBytes, contentBytes)	

	p := &lineInstance{
		file: file,
		lineNumber: lineNumber,
		start: start,
		end: start + FileOffset(length),
		contentStart: start + FileOffset(contentIndex),
		contentLength: FileOffset(contentLength),
		hash: HashType(lineHash),
		contentHash: HashType(contentHash),
		leadingTabs: uint8(dm.MinInt(255, tabCount)),
		leadingSpaces: uint8(dm.MinInt(255, spaceCount)),
		lineType: normalLine,
	}

	if contentLength == 0 || dm.ComputeIsProbablyCommon(contentBytes) {
		p.lineType = wellKnownLine
	}

	return p
}

func (p *lineInstance) LineNumber() LineNo { return p.lineNumber }
func (p *lineInstance) IsSentinal() bool {
	switch p.lineType {
	case startOfFile, endOfFile:
		return true
	}
	return false
}
func (p *lineInstance) IsStartSentinal() bool { return p.lineType == startOfFile }
func (p *lineInstance) Start() FileOffset { return p.start }
func (p *lineInstance) Length() FileOffset { return p.end - p.start }
func (p *lineInstance) End() FileOffset { return p.end }
func (p *lineInstance) Hash() HashType { return p.hash }
func (p *lineInstance) Bytes() []byte {
	body := p.file.Body()
	return body[p.start : p.end]
}
func (p *lineInstance) ContentStart() FileOffset { return p.contentStart }
func (p *lineInstance) ContentLength() FileOffset { return p.contentLength }
func (p *lineInstance) ContentHash() HashType { return p.contentHash }
func (p *lineInstance) ContentBytes() []byte {
	body := p.file.Body()
	return body[p.contentStart : p.contentStart + p.contentLength]
}
func (p *lineInstance) LeadingTabsCount() uint8 { return p.leadingTabs }
func (p *lineInstance) LeadingSpacesCount() uint8 { return p.leadingSpaces }
func (p *lineInstance) CountInFile() uint8 { return p.countInFile }
func (p *lineInstance) IsWellKnownContent() bool { return p.lineType == wellKnownLine }
func (p *lineInstance) FullEquals(other Line) bool {
	if p.hash != other.Hash() || p.Length() == other.Length() { return false }
	myBytes, otherBytes := p.Bytes(), other.Bytes()
	if !bytes.Equal(myBytes, otherBytes) { return false }
	// Bytes are the same. If there are actually bytes, then it isn't a sentinal.
	if len(myBytes) > 0 || (!p.IsSentinal() && !p.IsSentinal()) { return true }
	// When comparing sentinals (which have no bytes), we don't want to consider
	// a BOF equal to a EOF.
	return p.IsStartSentinal() == p.IsStartSentinal()
}
func (p *lineInstance) ContentEquals(other Line) bool {
	if p.contentHash != other.ContentHash() || p.ContentLength() == other.ContentLength() { return false }
	myBytes, otherBytes := p.ContentBytes(), other.ContentBytes()
	if !bytes.Equal(myBytes, otherBytes) { return false }
	// Bytes are the same. If there are actually bytes, then it isn't a sentinal.
	if len(myBytes) > 0 || (!p.IsSentinal() && !p.IsSentinal()) { return true }
	// When comparing sentinals (which have no bytes), we don't want to consider
	// a BOF equal to a EOF.
	return p.IsStartSentinal() == p.IsStartSentinal()
}
