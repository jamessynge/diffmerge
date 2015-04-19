package dm

import (
	"github.com/golang/glog"
)

// Represents a range of lines in a file. Intended to be used for computing
// the alignment of two files after having eliminated the common prefix and
// suffix lines of the files.

type FileRange interface {
	// Is the range made up of a sequence of adjacent lines (i.e. with no gaps,
	// and no repeats)?
	IsContiguous() bool

	// Is the FileRange empty (GetLineCount() == 0)?
	IsEmpty() bool

	// Returns the number of lines in the range.
	GetLineCount() int

	// Returns the index of the first line (zero for the whole file).
	GetStartLine() int

	// Returns the LinePos for the line at offset within this range (where zero
	// is the first line in the range).
	GetLinePosRelative(offsetInRange int) LinePos

	// Returns the hash of the line (full or normalized) at the offset within this
	// range (where zero is the first line in the range).
	GetLineHashRelative(offsetInRange int, normalized bool) uint32

	// Returns those lines for which fn returns true.
	Select(fn func(lp LinePos) bool) []LinePos

	// Returns the positions (line numbers, zero-based) within
	// the underlying file at which the full line hashes appear.
	GetHashPositions() map[uint32][]int

	// Returns the positions (line numbers, zero-based) within
	// the underlying file at which the normalized line hashes appear.
	GetNormalizedHashPositions() map[uint32][]int

	// Returns a new FileRange for the specified subset.
	GetSubRange(startOffsetInRange, length int) FileRange
}

func FileRangeIsEmpty(p FileRange) bool {
	if p == nil {
		return true
	}
	return p.IsEmpty()
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
	if start < 0 || start+length > file.GetLineCount() {
		glog.Fatalf("New range (%d, +%d) is invalid (max length %d)",
			start, length, file.GetLineCount())
	}
	return &fileRange{
		file:   file,
		start:  start,
		length: length,
		beyond: start + length,
	}
}

func (p *fileRange) GetLineCount() int {
	if p == nil {
		return 0
	}
	return p.length
}
func (p *fileRange) IsEmpty() bool      { return p == nil || p.GetLineCount() == 0 }
func (p *fileRange) IsContiguous() bool { return true }
func (p *fileRange) GetStartLine() int  { return p.start }

func (p *fileRange) GetLinePosRelative(offsetInRange int) LinePos {
	return p.file.Lines[p.start+offsetInRange]
}

func (p *fileRange) GetLineHashRelative(offsetInRange int, normalized bool) uint32 {
	lp := &p.file.Lines[p.start+offsetInRange]
	if normalized {
		return lp.NormalizedHash
	} else {
		return lp.Hash
	}
}

func (p *fileRange) GetHashPositions() map[uint32][]int {
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

func (p *fileRange) GetNormalizedHashPositions() map[uint32][]int {
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

func (p *fileRange) GetSubRange(startOffsetInRange, length int) FileRange {
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
