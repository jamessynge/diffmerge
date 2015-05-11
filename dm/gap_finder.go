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
	frp FileRangePair, blockPairs BlockPairs) (aRanges []FileRange, bRanges []FileRange) {
	glog.V(1).Infof("FindGapsInRangePair: %s", frp.BriefDebugString())
	SortBlockPairsByAIndex(blockPairs)
	fillPossibleGap := func(aLo, aHi, bLo, bHi int) {
		glog.V(1).Infof("fillPossibleGap A:[%d, %d)   B:[%d, %d)", aLo, aHi, bLo, bHi)
		if aLo >= aHi && bLo >= bHi {
			return
		}
		aLo, bLo = frp.ToRangeOffsets(aLo, bLo)
		aHi, bHi = frp.ToRangeOffsets(aHi, bHi)
		srp := frp.MakeSubRangePair(aLo, aHi-aLo, bLo, bHi-bLo)
		aRanges = append(aRanges, srp.ARange())
		bRanges = append(bRanges, srp.BRange())
	}
	aLo, bLo := frp.ToFileIndices(0, 0)
	glog.V(1).Infof("FindGapsInRangePair: first aLo=%d, bLo=%d", aLo, bLo)
	for n, bp := range blockPairs {
		glog.V(1).Infof("FindGapsInRangePair: blockPair[%d] = %v", n, bp)
		aHi, bHi := bp.AIndex, bp.BIndex
		if aLo <= aHi && bLo <= bHi {
			fillPossibleGap(aLo, aHi, bLo, bHi)
		} else if aLo < aHi || bLo < bHi {
			glog.Fatalf("Missed gap before pair #%d: A:[%d, %d),  B:[%d, %d)",
				n, aLo, aHi, bLo, bHi)
		}
		aLo = MaxInt(aLo, bp.ABeyond())
		bLo = MaxInt(bLo, bp.BBeyond())
		glog.V(1).Infof("FindGapsInRangePair: aLo=%d, bLo=%d", aLo, bLo)
	}
	aHi := frp.ARange().BeyondIndex()
	bHi := frp.BRange().BeyondIndex()
	glog.V(1).Infof("FindGapsInRangePair: final aHi=%d, bHi=%d", aHi, bHi)
	if aLo <= aHi && bLo <= bHi {
		fillPossibleGap(aLo, aHi, bLo, bHi)
	} else if aLo < aHi || bLo < bHi {
		glog.Fatalf("Missed gap after pair #%d: A:[%d, %d),  B:[%d, %d)",
			len(blockPairs)-1, aLo, aHi, bLo, bHi)
	}
	return
}

/*
// Creates two slices, aRanges and bRanges, each the same length. For each
// index N in the slices, at most one of the two slices will have a nil
// value.
func FindGapsInRangePairWithMoves(
	frp FileRangePair, blockPairs BlockPairs) (aRanges []FileRange, bRanges []FileRange) {
	glog.V(1).Infof("FindGapsInRangePairWithMoves: %s", frp.BriefDebugString())
	adjacencies := MakeBlockPairAdjacencies(blockPairs)






	SortBlockPairsByBIndex(blockPairs)
	pair2BIndex := blockPairs.MakeReverseIndex()
	SortBlockPairsByAIndex(blockPairs)
	fillPossibleGap := func(aLo, aHi, bLo, bHi int) {
		glog.V(1).Infof("fillPossibleGap A:[%d, %d)   B:[%d, %d)", aLo, aHi, bLo, bHi)
		if aLo >= aHi && bLo >= bHi {
			return
		}
		aLo, bLo = frp.ToRangeOffsets(aLo, bLo)
		aHi, bHi = frp.ToRangeOffsets(aHi, bHi)
		srp := frp.MakeSubRangePair(aLo, aHi-aLo, bLo, bHi-bLo)
		aRanges = append(aRanges, srp.ARange())
		bRanges = append(bRanges, srp.BRange())
	}
	aLo, bLo := frp.ToFileIndices(0, 0)
	glog.V(1).Infof("FindGapsInRangePairWithMoves: first aLo=%d, bLo=%d", aLo, bLo)
	for n, bp := range blockPairs {
		glog.V(1).Infof("FindGapsInRangePairWithMoves: blockPair[%d] = %v", n, bp)
		aHi, bHi := bp.AIndex, bp.BIndex
		if aLo <= aHi && bLo <= bHi {
			fillPossibleGap(aLo, aHi, bLo, bHi)
		} else if aLo < aHi || bLo < bHi {
			glog.Fatalf("Missed gap before pair #%d: A:[%d, %d),  B:[%d, %d)",
				n, aLo, aHi, bLo, bHi)
		}
		aLo = MaxInt(aLo, bp.ABeyond())
		bLo = MaxInt(bLo, bp.BBeyond())
		glog.V(1).Infof("FindGapsInRangePairWithMoves: aLo=%d, bLo=%d", aLo, bLo)
	}
	aHi := frp.ARange().BeyondIndex()
	bHi := frp.BRange().BeyondIndex()
	glog.V(1).Infof("FindGapsInRangePairWithMoves: final aHi=%d, bHi=%d", aHi, bHi)
	if aLo <= aHi && bLo <= bHi {
		fillPossibleGap(aLo, aHi, bLo, bHi)
	} else if aLo < aHi || bLo < bHi {
		glog.Fatalf("Missed gap after pair #%d: A:[%d, %d),  B:[%d, %d)",
			len(blockPairs)-1, aLo, aHi, bLo, bHi)
	}
	return
}
*/
