package dm

import (
	"fmt"

	"github.com/golang/glog"
)

// Represents a range of lines in a file. Intended to be used for computing
// the alignment of two files after having eliminated the common prefix and
// suffix lines of the files.

type FileRange interface {
	File() *File

	// Is the FileRange empty (GetLineCount() == 0)?
	IsEmpty() bool

	// Returns the number of lines in the range.
	Length() int

	// Returns the index of the first line (zero for the whole file).
	FirstIndex() int

	// Returns index of line immediately after the range.
	BeyondIndex() int

	// Returns the LinePos for the line at offset within this range (where zero
	// is the first line in the range).
	LinePosAtOffset(offsetInRange int) LinePos

	// Returns the hash of the line (full or normalized) at the offset within this
	// range (where zero is the first line in the range).
	LineHashAtOffset(offsetInRange int, normalized bool) uint32

	// Returns those lines for which fn returns true.
	Select(fn func(lp LinePos) bool) []LinePos

	// Returns the positions (line numbers, zero-based) within
	// the underlying file at which the full line hashes appear.
	HashPositions() map[uint32][]int

	// Returns the positions (line numbers, zero-based) within
	// the underlying file at which the normalized line hashes appear.
	NormalizedHashPositions() map[uint32][]int

	// Returns a FileRange for the specified subset.
	MakeSubRange(startOffsetInRange, length int) FileRange

	ToFileIndex(offsetInRange int) (indexInFile int)
	ToRangeOffset(indexInFile int) (offsetInRange int)
}

func FileRangeIsEmpty(p FileRange) bool {
	if p == nil {
		return true
	}
	return p.IsEmpty()
}

func FileRangeLength(p FileRange) int {
	if p == nil {
		return 0
	}
	return p.Length()
}

type fileRange struct {
	file *File

	start  int // First line in the range
	length int // Number of lines in the range
	beyond int // Line just beyond the range

	// TODO Consider whether the position value should be represented by the
	// position within the range (relative), or within the file (absolute).

	hashPositions           HashPositions // Position of different hashes in the range (absolute line numbers).
	normalizedHashPositions HashPositions // Position of different hashes in the range (absolute line numbers).
}

func CreateFileRange(file *File, start, length int) FileRange {
	return file.MakeSubRange(start, length)
}

func (p *fileRange) String() string {
	if p == nil {
		return "fileRange{nil}"
	}
	return fmt.Sprintf("fileRange{lines [%d, %d) of %s}", p.start, p.BeyondIndex(), p.file.BriefDebugString())
}

func (p *fileRange) Length() int      { return p.length }
func (p *fileRange) IsEmpty() bool    { return p == nil || p.Length() == 0 }
func (p *fileRange) FirstIndex() int  { return p.start }
func (p *fileRange) BeyondIndex() int { return p.start + p.length }

func (p *fileRange) LinePosAtOffset(offsetInRange int) LinePos {
	return p.file.Lines[p.start+offsetInRange]
}

func (p *fileRange) LineHashAtOffset(offsetInRange int, normalized bool) uint32 {
	lp := &p.file.Lines[p.start+offsetInRange]
	if normalized {
		return lp.NormalizedHash
	} else {
		return lp.Hash
	}
}

func (p *fileRange) HashPositions() map[uint32][]int {
	// Populate map if necessary.
	if len(p.hashPositions) == 0 {
		p.hashPositions = make(map[uint32][]int)
		for n := 0; n < p.length; n++ {
			hash := p.file.GetHashOfLine(p.start + n)
			p.hashPositions[hash] = append(p.hashPositions[hash], p.start+n)
		}
	}
	return p.hashPositions
}

func (p *fileRange) NormalizedHashPositions() map[uint32][]int {
	// Populate map if necessary.
	if len(p.normalizedHashPositions) == 0 {
		p.normalizedHashPositions = make(map[uint32][]int)
		for n := 0; n < p.length; n++ {
			hash := p.file.GetNormalizedHashOfLine(p.start + n)
			p.normalizedHashPositions[hash] = append(p.normalizedHashPositions[hash], p.start+n)
		}
	}
	return p.normalizedHashPositions
}

func (p *fileRange) Select(fn func(lp LinePos) bool) []LinePos {
	var result []LinePos
	for n := 0; n < p.length; n++ {
		lp := p.file.Lines[p.start+n]
		if fn(lp) {
			result = append(result, lp)
		}
	}
	return result
}

func (p *fileRange) MakeSubRange(startOffsetInRange, length int) FileRange {
	if startOffsetInRange < 0 || startOffsetInRange+length > p.length {
		glog.Fatalf("New range (%d, +%d) is invalid (max length %d)",
			startOffsetInRange, length, p.length)
	}
	if startOffsetInRange == 0 && length == p.length {
		return p
	}
	start := p.start + startOffsetInRange
	return &fileRange{
		file:   p.file,
		start:  start,
		length: length,
		beyond: start + length,
	}
}

func (p *fileRange) ToFileIndex(offsetInRange int) (indexInFile int) {
	// We assume here that File line indices start at 0, as do FileRange offsets.
	if 0 <= offsetInRange && offsetInRange <= p.length {
		return p.start + offsetInRange
	}
	glog.Fatalf("Offset %d is out of FileRange offsets [%d, %d)",
		offsetInRange, 0, p.length)
	return
}

func (p *fileRange) ToRangeOffset(indexInFile int) (offsetInRange int) {
	if p.start <= indexInFile && indexInFile <= p.beyond {
		return indexInFile - p.start
	}
	glog.Fatalf("Index %d is out of FileRange indices [%d, %d)",
		indexInFile, p.start, p.beyond)
	return
}

func (p *fileRange) File() *File {
	return p.file
}
