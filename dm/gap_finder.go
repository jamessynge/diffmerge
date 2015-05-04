package dm

// Given a file range, and BlockPairs representing matches in that range, find
// the gaps (represented as FileRanges).

import (
	"github.com/golang/glog"

)

// Creates two slices, aRanges and bRanges, each the same length. For each
// index N in the slices, at most one of the two slices will have a nil
// value.
func FindGapsInRangePair(
	frp FileRangePair, blockPairs []*BlockPair) (aRanges []FileRange, bRanges []FileRange) {
	SortBlockPairsByAIndex(blockPairs)
	fillPossibleGap := func(aLo, aHi, bLo, bHi int) {
		if aLo >= aHi && bLo >= bHi { return }
		aLo, bLo = frp.ToRangeOffsets(aLo, bLo)
		aHi, bHi = frp.ToRangeOffsets(aHi, bHi)
		srp := frp.MakeSubRangePair(aLo, aHi - aLo, bLo, bHi - bLo)
		aRanges = append(aRanges, srp.ARange())
		bRanges = append(bRanges, srp.BRange())
	}
	aLo, bLo := frp.ToFileIndices(0, 0)
	for n, bp := range blockPairs {
		aHi, bHi := bp.AIndex, bp.BIndex
		if aLo <= aHi && bLo <= bHi {
			fillPossibleGap(aLo, aHi, bLo, bHi)
		} else if aLo < aHi || bLo < bHi {
			glog.Fatalf("Missed gap before pair #%d: A:[%d, %d),  B:[%d, %d)",
					n, aLo, aHi, bLo, bHi)
		}
		aLo = MaxInt(aLo, bp.ABeyond())
		bLo = MaxInt(bLo, bp.BBeyond())
	}
	aHi := frp.ARange().BeyondIndex()
	bHi := frp.BRange().BeyondIndex()
	if aLo <= aHi && bLo <= bHi {
		fillPossibleGap(aLo, aHi, bLo, bHi)
	} else if aLo < aHi || bLo < bHi {
		glog.Fatalf("Missed gap after pair #%d: A:[%d, %d),  B:[%d, %d)",
			len(blockPairs) - 1, aLo, aHi, bLo, bHi)
	}
	return
}
