package dm

import ()

// Basic assumption: there won't be more unique lines or lines in a file
// than can be counted as positive int.
// Basic assumption 2 (PROBABLY BAD): the hash function used won't have any
// collisions in real files. If this turns
// out to be a problem in practice (need to add detection code), then
// I'll change things.

type LinePos struct {
	Start, Length, Index int
	Hash                 uint32
	// TODO Add a hash for a "normalized" version of the line, with the thought
	// that if there is a very large amount of difference between two files, it
	// maybe due to relatively minor formatting changes (e.g. indentation or
	// justification) rather than other kinds of changes.
	// Possible normalizations:
	// * leading and trailing whitespace removed
	// * all interior whitespace runs collapsed to a single space
	//   or maybe completely removed
	// * convert all letters characters to a single case (very aggressive)
}

type File struct {
	Name  string    // Command line arg
	Body  []byte    // Body of the file
	Lines []LinePos // Locations and hashes of the file lines.

	// Counts is in support of Patience Diff, where we want to know which lines
	// are unique (or later, maybe want to find "relatively" unique lines).
	// Definitely assuming here that we don't have hash collisions.
	Counts map[uint32]int // Count of hash occurrences in file.
}

// Represents a match between files A and B.
type BlockMatch struct {
	// Index is same as LinePos.Index of starting line of match.
	// Length is number of lines that match.
	AIndex, BIndex, Length int
}

// Represents a pairing of ranges in files A and B, primarily for output,
// as we can produce different pairings based on which file we consider
// primary (i.e. in the face of block moves we may print A in order, but
// B out of order).
type BlockPair struct {
	AIndex, ALength int
	BIndex, BLength int
	IsMatch         bool
	IsMove          bool // Does this represent a move?
}
