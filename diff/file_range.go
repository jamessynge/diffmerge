package diff

import (
	_ "github.com/golang/glog"
)

// Represents a range of lines in a file. Intended to be used for computing
// the alignment of two files after having eliminated the common prefix and
// suffix lines of the files.

type FileRange interface {
	// The containing File.
	File() File

	// Containing FileRange, if this was constructed from another FileRange.
	Parent() FileRange

	// Is the FileRange empty (GetLineCount() == 0)?
	IsEmpty() bool

	// Returns the number of lines in the range (including sentinal(s), if the
	// range goes all the way to the end(s) of the file).
	LineCount() LineNo

	// Returns the line number of the first line in the range.
	StartLineNumber() LineNo

	// Returns the Line of the File whose LineNumber matches the argument,
	// or nil if there is none.
	Line(lineNumber LineNo) Line

	// Returns the LinePos for the line at offsetInRange (where zero
	// is the first line in the range).
	LineRelative(offsetInRange int) Line

	// Returns those lines for which fn returns true.
	Select(fn func(line Line) bool) []Line

	// Returns a FileRange for the specified subset.
	GetSubRange(startOffsetInRange, length int) FileRange
}

func FileRangeIsEmpty(p FileRange) bool {
	if p == nil {
		return true
	}
	return p.IsEmpty()
}
