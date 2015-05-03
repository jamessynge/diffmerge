package dm

import (

)

////////////////////////////////////////////////////////////////////////////////

type FilePair interface {
	AFile() *File
	BFile() *File

	ALength() int
	BLength() int

	BriefDebugString() string

	CompareLines(aIndex, bIndex int, maxRareOccurrences uint8) (equal, approx, rare bool)

	MakeSubRangePair(aIndex, aLength, bIndex, bLength int) *FileRangePair
}

type filePair struct {
	aFile, bFile *File


}



