package dm

import ()

// Given 3 files where both yours and theirs are different from base, compute a
// 3-way diff, analogous to diff3. The goal is to identify those blocks that
// are unchanged, are different in only one file, or are changed in both yours
// and theirs. No attempt is made to resolve the differences. Every BlockPair
// in the inputs will appear in at least one of the Diff3Triples return,
// possibly more than one (based on the alignment of changes/moves/conflicts).
func PerformDiff3(
	yours, base, theirs *File, b2yPairs, b2tPairs BlockPairs,
	cfg DifferencerConfig) (triples Diff3Triples, conflictsExist bool) {
	SortBlockPairsByAIndex(b2yPairs)
	SortBlockPairsByAIndex(b2tPairs)

	return
}

type Diff3TripleType int

const (
	UnchangedTriple Diff3TripleType = iota
	YoursChangedTriple
	TheirsChangedTriple
	BothSameTriple
	ConflictTriple
)

type Diff3Triple struct {
	TripleType Diff3TripleType

	// The lines in base that are the anchor for this block.
	BaseStart, BaseBeyond int

	// The BlockPairs that are the basis of this triple.
	B2YPair, B2TPair *BlockPair
}
type Diff3Triples []*Diff3Triple
