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
	AIndex, ALength   int
	BIndex, BLength   int
	MoveId            int // An attempt to tell one move from another.
	IsMatch           bool
	IsNormalizedMatch bool
	IsMove            bool // Does this represent a move?
}

// Is p immediately before o, in both A and B.
func BlockPairsAreInOrder(p, o *BlockPair) bool {
	nextA := p.AIndex + p.ALength
	nextB := p.BIndex + p.BLength
	return nextA == o.AIndex && nextB == o.BIndex
}

// Is p before o, in both A and B?
func BlockPairsLess(p, o *BlockPair) bool {
	nextA := p.AIndex + p.ALength
	nextB := p.BIndex + p.BLength
	return nextA <= o.AIndex && nextB <= o.BIndex
}

// TODO Experiment with how block COPIES are handled; Not sure these are
// remotely correct yet.

func BlockPairsAreSameType(p, o *BlockPair) bool {
	return p.IsMatch == o.IsMatch && p.IsNormalizedMatch == o.IsNormalizedMatch
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

		if BlockPairsAreSameType(j, k) && BlockPairsAreInOrder(j, k) && !IsSentinal(j) && !IsSentinal(k) {
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
