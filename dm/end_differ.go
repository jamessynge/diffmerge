package dm

import (
	"github.com/golang/glog"
)

// Used for removing the shared ends from files before computing diff of the
// non-matching middle.

type middleAndSharedEnds struct {
	baseRangePair FileRangePair

	sharedEndsData SharedEndsData

	sharedPrefixPairs []*BlockPair
	sharedSuffixPairs []*BlockPair

	middleRangePair FileRangePair
}

func FindMiddleAndSharedEnds(frp FileRangePair, config DifferencerConfig) *middleAndSharedEnds {
	rareEndsOnly := config.OmitProbablyCommonLines
	maxRareOccurrences := uint8(MaxInt(1, MinInt(255, config.MaxRareLineOccurrencesInFile)))
	onlyExactMatches := !config.MatchNormalizedEnds

	sharedEndsData := frp.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)

	if !sharedEndsData.HasPrefixOrSuffix() {
		glog.Infof("Found no shared prefix or suffix in %s", frp.BriefDebugString())
		return nil
	} else if rareEndsOnly && !sharedEndsData.HasRarePrefixOrSuffix() {
		glog.Infof("Found no rare, shared prefix or suffix in %s", frp.BriefDebugString())
		return nil
	}

	// Handle overlapping shared prefix and suffix by bailing, claiming in essence
	// that there aren't shared prefix and suffix lines, when there are; but that
	// then allows us to apply other techniques to resolve the matter (e.g.
	// weighted LCS).
	if sharedEndsData.PrefixAndSuffixOverlap(rareEndsOnly) {
		glog.Infof("Found overlapping prefix and suffix in %s", frp.BriefDebugString())
		return nil
	}

	glog.V(1).Info("FindMiddleAndSharedEnds - calling frp.MakeMiddleRangePair")
	middleFRP := frp.MakeMiddleRangePair(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)

	glog.V(1).Info("FindMiddleAndSharedEnds - calling frp.MakeSharedEndBlockPairs")
	prefixPairs, suffixPairs := frp.MakeSharedEndBlockPairs(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)

	return &middleAndSharedEnds{
		baseRangePair: frp,
		sharedEndsData: sharedEndsData,
		sharedPrefixPairs: prefixPairs,
		sharedSuffixPairs: suffixPairs,
		middleRangePair: middleFRP,
	}
}

/*

func MakeSimpleDiffer(baseRangePair FileRangePair) *endDiffer {
	p := &endDiffer{
		baseRangePair: baseRangePair,
	}
	return p
}

// Note that common prefix may overlap, as when comparing these two strings
// for common prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *endDiffer) MeasureCommonEnds(onlyExactMatches bool, maxRareOccurrences uint8) SharedEndsData {
	return p.baseRangePair.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
}

//func (p *endDiffer) HasCommonEnds(rareEndsOnly bool) bool {
//	if rareEndsOnly {
//		return p.baseRangePair.HasRarePrefixOrSuffix()
//	} else {
//		return p.baseRangePair.HasPrefixOrSuffix()
//	}
//}

func (p *endDiffer) SetMiddleToBase() (bothNonEmpty bool) {
	p.middleRangePair = p.baseRangePair
	return p.MiddleRangesAreNotEmpty()
}

func (p *endDiffer) SetMiddleToGap(
		rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) {
	p.middleRangePair = p.baseRangePair.MakeMiddleRangePair(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)
	p.sharedPrefixPairs, p.sharedSuffixPairs = p.baseRangePair.MakeSharedEndBlockPairs(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)
}

func (p *endDiffer) BaseRangesAreNotEmpty() bool {
	return p.baseRangePair.BothAreNotEmpty()
}

func (p *endDiffer) MiddleRangesAreNotEmpty() bool {
	return p.middleRangePair.BothAreNotEmpty()
}

func (p *endDiffer) ConvertBaseOffsets(aOffset, bOffset int) (aIndex, bIndex int) {
	return p.baseRangePair.ToFileIndices(aOffset, bOffset)
}

func (p *endDiffer) ConvertMiddleOffsets(aMiddleOffset, bMiddleOffset int) (aIndex, bIndex int) {
	return p.middleRangePair.ToFileIndices(aMiddleOffset, bMiddleOffset)
}

func (p *endDiffer) ComputeWeightedLCSOfMiddle(s SimilarityFactors) (score float32) {
	p.middlePairs, score = p.middleRangePair.ComputeWeightedLCSBlockPairs(s)
	return score
}

func (p *endDiffer) FillGapsWithEasyMatches() {
	pairs := p.middlePairs
	limit := len(pairs)
	if limit < 2 {
		return
	}
	SortBlockPairsByAIndex(pairs)
	fp := p.middleRangePair.BaseFilePair()
	prevPair := pairs[0]
	for i := 1; i < limit; i++ {
		thisPair := pairs[i]
		equal, approx := fp.CanFillGapWithMatches(prevPair, thisPair)
		if equal || approx {
			aLo, bLo := prevPair.ABeyond(), prevPair.BBeyond()
			aHi, bHi := thisPair.AIndex, thisPair.BIndex
			newPair := &BlockPair{
				AIndex:            aLo,
				ALength:           aHi - aLo,
				BIndex:            bLo,
				BLength:           bHi - bLo,
				IsMatch:           equal,
				IsNormalizedMatch: !equal,
			}
			p.middlePairs = append(p.middlePairs, newPair)
		}
		prevPair = thisPair
	}
	if len(p.middlePairs) > limit {
		SortBlockPairsByAIndex(p.middlePairs)
	}
}


//func (p *endDiffer) CompareBaseLines(aOffset, bOffset int) (equal, approx, rare bool) {
//	return p.CompareFileLines(p.ConvertBaseOffsets(aOffset, bOffset))
//}
//
//func (p *endDiffer) CompareMiddleLines(aMiddleOffset, bMiddleOffset int) (equal, approx, rare bool) {
//	return p.CompareFileLines(p.ConvertMiddleOffsets(aMiddleOffset, bMiddleOffset))
//}

/*
func (p *endDiffer) SetMiddlePairsFromIndexPairs(
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
