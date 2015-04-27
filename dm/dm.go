package dm

import ()

// Basic assumption: there won't be more unique lines or lines in a file
// than can be counted as positive int.
// Basic assumption 2 (PROBABLY BAD): the hash function used won't have any
// collisions in real files. If this turns
// out to be a problem in practice (need to add detection code), then
// I'll change things.

// TODO Introduce new interfaces so that we can do coarse grained matching
// (line at a time), medium grained matching (words/symbols/whitespace) or
// fine grained matching (characters), all with the same algorithms. For
// example:
//
// Atom: unit of matching
// AtomSequence:
//    collection of Atoms maintaining some order, possibly not ordered
//    by position in file, and possibly not covering the full file.
//    (allow repeats?)
// AtomString:
//    collection of unique Atoms in the same order they appear in the
//    file, and where adjacent atoms in the string are adjacent in the file.
// AtomStringTree:
//    collection of AtomStrings in the same order as their constituent
//    atoms.
//
// Since it would take more work to diff entire files at the character level,
// we'd instead diff lines initially to get an alignment (e.g. which lines
// have been copied, moved, deleted, inserted or changed, and which lines are
// unmodified); after that we can focus on the areas of change at the word
// or character level.

type LinePos struct {
	Start, Length, Index int
	// Hash of the full line (including newline and/or carriage return at end).
	Hash uint32
	// Hash for a "normalized" version of the line, with the thought
	// that if there is a very large amount of difference between two files, it
	// maybe due to relatively minor formatting changes (e.g. indentation or
	// justification) rather than other kinds of changes.
	// Possible normalizations:
	// * leading and trailing whitespace removed
	// * all interior whitespace runs collapsed to a single space
	//   or maybe completely removed
	// * convert all letters characters to a single case (very aggressive)
	NormalizedHash uint32

	// Count of the normalized hash in the file.
	// Maximum is 255, but that is OK for rare-ness checking.
	CountInFile uint8

	// Length of line after normalization.
	// Maximum is 255, but that is OK for deciding whether a line is short or not.
	NormalizedLength uint8

	// Is this a well known common line (e.g. "/*" or "#", or an empty line).
	ProbablyCommon bool // Based solely on normalized content, not other lines.
}

type IndexPair struct {
	Index1, Index2 int
}

// Define BlockMatches U and V in matches to be "adjacent matches"
// when there exist integers i and j such that:
//      matchesByA[i] == U      matchesByA[i + 1] == V
//      matchesByB[j] == U      matchesByB[j + 1] == V
// We want to identify such adjacent matches because we can then be
// confident in emiting a conflict, insertion or deletion between
// two, rather than there being a likelihood that the gap between the
// two represents a move.

// Represents a match between files A and B.
type BlockMatch struct {
	// Index is same as LinePos.Index of starting line of match.
	// Length is number of lines that match.
	AIndex, BIndex, Length int
}
