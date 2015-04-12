package dm

import ()

// Represents a range of lines in one file. Intended to be used for computing
// the alignment of two files after having eliminated the common prefix and
// suffix lines of the files.
type FileRange struct {
	file *File

	start  int // First line in the range
	length int // Number of lines in the range
	beyond int // Line just beyond the range

	// TODO Consider whether the position value should be represented by the
	// position within the range (relative), or within the file (absolute).

	hashPositions           HashPositions // Position of different hashes in the range (absolute line numbers).
	normalizedHashPositions HashPositions // Position of different hashes in the range (absolute line numbers).
}

func CreateFileRange(file *File, start, length int) *FileRange {
	return &FileRange{
		file:   file,
		start:  start,
		length: length,
		beyond: start + length,
	}
}

func (p *FileRange) GetHashPositions() map[uint32][]int {
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

func (p *FileRange) GetNormalizedHashPositions() map[uint32][]int {
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

func (p *FileRange) Select(fn func(lp LinePos) bool) []LinePos {
	var result []LinePos
	for n := 0; n < p.length; n++ {
		lp := p.file.Lines[p.start+n]
		if fn(lp) {
			result = append(result, lp)
		}
	}
	return result
}
