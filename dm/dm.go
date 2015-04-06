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

type BlockMove struct {
	AOffset, BOffset, Length int
}
