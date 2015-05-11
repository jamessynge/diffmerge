package dm

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

var _ = spew.Dump

// Represents a pairing of ranges in files A and B, primarily for output,
// as we can produce different pairings based on which file we consider
// primary (i.e. in the face of block moves we may print A in order, but
// B out of order).
type BlockPair struct {
	AIndex, ALength int
	BIndex, BLength int
	MoveId          int // An attempt to tell one move from another.
	// If IsMatch and IsNormalizedMatch are both true, this means that the
	// lines match after normalization, and it is possible that some or even
	// all of them are exact mathes, but we've not recorded that.
	IsMatch           bool
	IsNormalizedMatch bool
	IsMove            bool // Does this represent a move?
}

func IsSentinal(p *BlockPair) bool {
	return p.AIndex < 0 || (p.ALength == 0 && p.BLength == 0)
}

func (p *BlockPair) markAsIdenticalMatch() {
	p.IsMatch = true
	p.IsNormalizedMatch = false
}

func (p *BlockPair) markAsNormalizedMatch() {
	p.IsMatch = false
	p.IsNormalizedMatch = true
}

func (p *BlockPair) markAsMismatch() {
	p.IsMatch = false
	p.IsNormalizedMatch = false
}

func (p *BlockPair) ABeyond() int {
	return p.AIndex + p.ALength
}

func (p *BlockPair) BBeyond() int {
	return p.BIndex + p.BLength
}

func (p *BlockPair) IsSentinal() bool {
	return p.AIndex < 0 || (p.ALength == 0 && p.BLength == 0)
}

// Is p immediately before o, in both A and B.
func BlockPairsAreNeighbors(p, o *BlockPair) bool {
	return p.ABeyond() == o.AIndex && p.BBeyond() == o.BIndex
}

// Is p before o, in both A and B?
func BlockPairsLess(p, o *BlockPair) bool {
	return p.ABeyond() <= o.AIndex && p.BBeyond() <= o.BIndex
}

func BlockPairsAreSameType(p, o *BlockPair) bool {
	return p.IsMatch == o.IsMatch && p.IsNormalizedMatch == o.IsNormalizedMatch
}

////////////////////////////////////////////////////////////////////////////////

type BlockPairs []*BlockPair

func (s BlockPairs) Len() int      { return len(s) }
func (s BlockPairs) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s BlockPairs) IsInStrictOrder() bool {
	for n, limit := 1, len(s); n < limit; n++ {
		if !BlockPairsLess(s[n-1], s[n]) {
			return false
		}
	}
	return true
}
func (s BlockPairs) LimitIndexPairs() (limitsInA, limitsInB IndexPair) {
	length := len(s)
	if length == 0 {
		return
	}
	first, last := s[0], s[length-1]
	limitsInA.Index1 = first.AIndex
	limitsInA.Index2 = last.ABeyond()
	limitsInB.Index1 = first.BIndex
	limitsInB.Index2 = last.BBeyond()
	return
}
func (s BlockPairs) CountLinesInPairs() (numALines, numBLines int) {
	for _, pair := range s {
		numALines += pair.ALength
		numBLines += pair.BLength
	}
	return
}
func (s BlockPairs) MakeReverseIndex() (pair2Index map[*BlockPair]int) {
	pair2Index = make(map[*BlockPair]int)
	for n, pair := range s {
		if m, ok := pair2Index[pair]; ok {
			glog.Fatalf("BlockPair is present in slice twice, at indices %d and %d\nBlockPair: %#v",
				m, n, pair)
		}
		pair2Index[pair] = n
	}
	return
}

// Each time we identify a move, we label it with a unique id.
var lastMoveId int

func (s BlockPairs) AssignMoveId() {
	lastMoveId++
	for _, pair := range s {
		pair.MoveId = lastMoveId
	}
}

////////////////////////////////////////////////////////////////////////////////

type BlockPairAdjacency struct {
	thePair                        *BlockPair
	sortedByAIndex, sortedByBIndex int
	// Note that prev or next pairs may overlap this pair.
	prevInA, nextInA *BlockPair
	prevInB, nextInB *BlockPair
}

func MakeBlockPairAdjacencies(blockPairs BlockPairs) (
	adjacencies map[*BlockPair]*BlockPairAdjacency) {
	adjacencies = make(map[*BlockPair]*BlockPairAdjacency)
	SortBlockPairsByBIndex(blockPairs)
	var prevPair *BlockPair
	for n, pair := range blockPairs {
		if m, ok := adjacencies[pair]; ok {
			glog.Fatalf("BlockPair is present in slice twice, at indices %d and %d\n"+
				"BlockPair: %#v", m, n, pair)
		}
		adjacencies[pair] = &BlockPairAdjacency{
			thePair:        pair,
			sortedByBIndex: n,
			prevInB:        prevPair,
		}
		if prevPair != nil {
			adjacencies[prevPair].nextInB = pair
		}
		prevPair = pair
	}
	prevPair = nil
	SortBlockPairsByAIndex(blockPairs)
	for n, pair := range blockPairs {
		adj := adjacencies[pair]
		adj.sortedByAIndex = n
		if prevPair != nil {
			adj.prevInA = prevPair
			adjacencies[prevPair].nextInA = pair
		}
		prevPair = pair
	}
	return
}

// Sort by AIndex or BIndex before calling CombineBlockPairs.
func CombineBlockPairs(sortedInput []*BlockPair) (output []*BlockPair) {
	if glog.V(1) {
		glog.Info("CombineBlockPairs entry")
		for n, pair := range sortedInput {
			glog.Infof("CombineBlockPairs sortedInput[%d] = %v", n, pair)
		}
	}

	output = append(output, sortedInput...)
	// For each pair of consecutive BlockPairs, if they can be combined,
	// combine them into the first of them.
	u, v, limit, removed := 0, 1, len(output), 0
	for v < limit {
		j, k := output[u], output[v]

		glog.Infof("CombineBlockPairs output[u=%d] = %v", u, j)
		glog.Infof("CombineBlockPairs output[v=%d] = %v", v, k)

		if BlockPairsAreSameType(j, k) && BlockPairsAreNeighbors(j, k) && !IsSentinal(j) && !IsSentinal(k) {
			glog.Infof("Combining BlockPairs:\n[%d]: %v\n[%d]: %v", u, *j, v, *k)
			j.ALength += k.ALength
			j.BLength += k.BLength
			output[v] = nil
			removed++
		} else {
			// BlockPairs can't be combined.
			output[u+1] = k
			u++
		}
		v++
	}
	glog.Infof("Removed %d (= %d) BlockPairs", v-u-1, removed)

	output = output[0 : u+1]

	if glog.V(1) {
		glog.Info("CombineBlockPairs exit")
		for n, pair := range output {
			glog.Infof("CombineBlockPairs output[%d] = %v", n, pair)
		}
	}

	return output
}
