package dm

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

type lcsOfFileRangePair struct {
	fileRangePair   FileRangePair
	lcsPairs        BlockPairs
	lcsScore        float32
	limitsInA       IndexPair
	limitsInB       IndexPair
	numMatchedLines int
}

func (p *lcsOfFileRangePair) AExtent() int {
	aIndex := p.lcsPairs[0].AIndex
	aBeyond := p.lcsPairs[len(p.lcsPairs)-1].ABeyond()
	return aBeyond - aIndex
}

func (p *lcsOfFileRangePair) BExtent() int {
	bIndex := p.lcsPairs[0].BIndex
	bBeyond := p.lcsPairs[len(p.lcsPairs)-1].BBeyond()
	return bBeyond - bIndex
}

// Compute Longest Common Subsequence of lines in two file ranges.  Returns
// nil if there is no match at all (i.e. the LCS is empty).
func PerformLCS(fileRangePair FileRangePair, config DifferencerConfig, sf SimilarityFactors) *lcsOfFileRangePair {
	glog.Infof("PerformLCS - DifferencerConfig:\n%s\n\nSimilarityFactors:\n%s\n",
		spew.Sdump(config), spew.Sdump(sf))
	lcsPairs, score := WeightedLCSBlockPairsOfRangePair(fileRangePair, sf)
	glog.Infof("PerformLCS - score: %v", score)
	if len(lcsPairs) == 0 {
		return nil
	}
	mayHaveMatchableGaps := len(lcsPairs) > 1 && (sf.NormalizedNonRare == 0 ||
		sf.ExactNonRare == 0 ||
		sf.NormalizedRare == 0 ||
		sf.ExactRare == 0)
	glog.Infof("PerformLCS - mayHaveMatchableGaps: %v", mayHaveMatchableGaps)
	if mayHaveMatchableGaps {
		// Fill the gaps.
		filledGaps := FillGapsWithEasyMatches(fileRangePair, lcsPairs)
		if len(filledGaps) > 0 {
			lcsPairs = append(lcsPairs, filledGaps...)
			SortBlockPairsByAIndex(lcsPairs)
		}
	}
	lcsData := &lcsOfFileRangePair{
		fileRangePair: fileRangePair,
		lcsPairs:      BlockPairs(lcsPairs),
		lcsScore:      score,
	}
	lcsData.limitsInA, lcsData.limitsInB = lcsData.lcsPairs.LimitIndexPairs()
	lcsData.numMatchedLines, _ = lcsData.lcsPairs.CountLinesInPairs()
	return lcsData
}

func FillGapsWithEasyMatches(frp FileRangePair, blockPairs []*BlockPair) (filledGaps []*BlockPair) {
	limit := len(blockPairs)
	if limit < 2 {
		return
	}
	SortBlockPairsByAIndex(blockPairs)
	prevPair := blockPairs[0]
	fp := frp.BaseFilePair()
	for i := 1; i < limit; i++ {
		thisPair := blockPairs[i]
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
			filledGaps = append(filledGaps, newPair)
		}
		prevPair = thisPair
	}
	return
}

/*

func (p *lcsDiffer) ComputeWeightedLCSOfMiddle(s SimilarityFactors) (score float32) {
	p.middlePairs, score = p.middleRangePair.ComputeWeightedLCSBlockPairs(s)
	return score

}



func MakeSimpleDiffer(baseRangePair FileRangePair) *lcsDiffer {
	p := &lcsDiffer{
		baseRangePair: baseRangePair,
	}
	return p
}

// Note that common prefix may overlap, as when comparing these two strings
// for common prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *lcsDiffer) MeasureCommonEnds(onlyExactMatches bool, maxRareOccurrences uint8) SharedEndsData {
	return p.baseRangePair.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
}

//func (p *lcsDiffer) HasCommonEnds(rareEndsOnly bool) bool {
//	if rareEndsOnly {
//		return p.baseRangePair.HasRarePrefixOrSuffix()
//	} else {
//		return p.baseRangePair.HasPrefixOrSuffix()
//	}
//}

func (p *lcsDiffer) SetMiddleToBase() (bothNonEmpty bool) {
	p.middleRangePair = p.baseRangePair
	return p.MiddleRangesAreNotEmpty()
}

func (p *lcsDiffer) SetMiddleToGap(
		rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) {
	p.middleRangePair = p.baseRangePair.MakeMiddleRangePair(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)
	p.sharedPrefixPairs, p.sharedSuffixPairs = p.baseRangePair.MakeSharedEndBlockPairs(
		rareEndsOnly, onlyExactMatches, maxRareOccurrences)
}

func (p *lcsDiffer) BaseRangesAreNotEmpty() bool {
	return p.baseRangePair.BothAreNotEmpty()
}

func (p *lcsDiffer) MiddleRangesAreNotEmpty() bool {
	return p.middleRangePair.BothAreNotEmpty()
}

func (p *lcsDiffer) ConvertBaseOffsets(aOffset, bOffset int) (aIndex, bIndex int) {
	return p.baseRangePair.ToFileIndices(aOffset, bOffset)
}

func (p *lcsDiffer) ConvertMiddleOffsets(aMiddleOffset, bMiddleOffset int) (aIndex, bIndex int) {
	return p.middleRangePair.ToFileIndices(aMiddleOffset, bMiddleOffset)
}

func (p *lcsDiffer) ComputeWeightedLCSOfMiddle(s SimilarityFactors) (score float32) {
	p.middlePairs, score = p.middleRangePair.ComputeWeightedLCSBlockPairs(s)
	return score
}


//func (p *lcsDiffer) CompareBaseLines(aOffset, bOffset int) (equal, approx, rare bool) {
//	return p.CompareFileLines(p.ConvertBaseOffsets(aOffset, bOffset))
//}
//
//func (p *lcsDiffer) CompareMiddleLines(aMiddleOffset, bMiddleOffset int) (equal, approx, rare bool) {
//	return p.CompareFileLines(p.ConvertMiddleOffsets(aMiddleOffset, bMiddleOffset))
//}

/*
func (p *lcsDiffer) SetMiddlePairsFromIndexPairs(
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
