package dm

import (
	_ "github.com/golang/glog"
)

// Derived from diffState in diff.go, supports operations on a pair of ranges,
// but not multiple passes.

type simpleDiffer struct {
	baseRangePair, middleRangePair *FileRangePair

	sharedPrefixPairs []*BlockPair
	sharedSuffixPairs []*BlockPair

	// Matches (presumably, not expecting to store mismatches in here) found in
	// the middle range.
	middlePairs []*BlockPair
}

func MakeSimpleDiffer(baseRangePair *FileRangePair) *simpleDiffer {
	p := &simpleDiffer{
		baseRangePair: baseRangePair,
	}
	return p
}

// Note that common prefix may overlap, as when comparing these two strings
// for common prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *simpleDiffer) MeasureCommonEnds(onlyExactMatches bool, maxRareOccurrences uint8) SharedEndsData {
	return p.baseRangePair.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
}

//func (p *simpleDiffer) HasCommonEnds(rareEndsOnly bool) bool {
//	if rareEndsOnly {
//		return p.baseRangePair.HasRarePrefixOrSuffix()
//	} else {
//		return p.baseRangePair.HasPrefixOrSuffix()
//	}
//}

func (p *simpleDiffer) SetMiddleToBase() (bothNonEmpty bool) {
	p.middleRangePair = p.baseRangePair
	return p.MiddleRangesAreNotEmpty()
}

func (p *simpleDiffer) SetMiddleToGap(
		rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) {
	p.middleRangePair = p.baseRangePair.MakeMiddleRangePair(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)
	p.sharedPrefixPairs, p.sharedSuffixPairs = p.baseRangePair.MakeSharedEndBlockPairs(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)
}

func (p *simpleDiffer) BaseRangesAreNotEmpty() bool {
	return p.baseRangePair.BothAreNotEmpty()
}

func (p *simpleDiffer) MiddleRangesAreNotEmpty() bool {
	return p.middleRangePair.BothAreNotEmpty()
}

func (p *simpleDiffer) ConvertBaseOffsets(aOffset, bOffset int) (aIndex, bIndex int) {
	return p.baseRangePair.ToFileIndices(aOffset, bOffset)
}

func (p *simpleDiffer) ConvertMiddleOffsets(aMiddleOffset, bMiddleOffset int) (aIndex, bIndex int) {
	return p.middleRangePair.ToFileIndices(aMiddleOffset, bMiddleOffset)
}

func (p *simpleDiffer) ComputeWeightedLCSOfMiddle(s SimilarityFactors) (score float32) {
	p.middlePairs, score = p.middleRangePair.ComputeWeightedLCSBlockPairs(s)
	return score
}

//func (p *simpleDiffer) CompareBaseLines(aOffset, bOffset int) (equal, approx, rare bool) {
//	return p.CompareFileLines(p.ConvertBaseOffsets(aOffset, bOffset))
//}
//
//func (p *simpleDiffer) CompareMiddleLines(aMiddleOffset, bMiddleOffset int) (equal, approx, rare bool) {
//	return p.CompareFileLines(p.ConvertMiddleOffsets(aMiddleOffset, bMiddleOffset))
//}

/*
func (p *simpleDiffer) SetMiddlePairsFromIndexPairs(
	matchingMiddleOffsets []IndexPair, matchedNormalizedLines bool) {
	if len(p.middlePairs) > 0 {
		glog.Fatalf("There are already %d middle pairs!", len(p.middlePairs))
	}
	// Assuming here that there are no moves (relative to aMiddleRange and bMiddleRange.
	SortIndexPairsByIndex1(matchingMiddleOffsets)
	// Convert these to BlockPair(s).
	var pair *BlockPair
	for i, m := range matchingMiddleOffsets {
		glog.V(1).Infof("matchingMiddleOffsets[%d] = %v", i, m)
		aIndex, bIndex := p.ConvertMiddleOffsets(m.Index1, m.Index2)
		isExactMatch := true
		if matchedNormalizedLines {
			isExactMatch, _, _ = p.CompareFileLines(aIndex, bIndex)
		}
		// Can we just grow the current BlockPair?
		if pair != nil && pair.IsMatch == isExactMatch &&
			pair.ABeyond() == aIndex &&
			pair.BBeyond() == bIndex {
			// Yes, so just increase the length.
			glog.V(1).Info("Growing BlockPair")
			pair.ALength++
			pair.BLength++
			continue
		}
		// Create a new pair.
		pair = &BlockPair{
			AIndex:            aIndex,
			ALength:           1,
			BIndex:            bIndex,
			BLength:           1,
			IsMatch:           isExactMatch,
			IsNormalizedMatch: !isExactMatch,
		}
		p.middlePairs = append(p.middlePairs, pair)
	}
	glog.Infof("Added %d middle pairs from %d matching lines", len(p.middlePairs), len(matchingMiddleOffsets))
}
*/
