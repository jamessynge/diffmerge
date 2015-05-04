package dm

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

var TO_BE_DELETED = glog.CopyStandardLogTo

// Aim here is to be able to represent two files, or two ranges, one in each
// of two files, as a single object.

type FileRangePair interface {
	// The files.
	BaseFilePair() FilePair

	ARange() FileRange
	BRange() FileRange

	ALength() int
	BLength() int

	ToFileIndices(aOffset, bOffset int) (aIndex, bIndex int)
	ToRangeOffsets(aIndex, bIndex int) (aOffset, bOffset int)
	MakeSubRangePair(aIndex, aLength, bIndex, bLength int) FileRangePair

	BriefDebugString() string

	MeasureSharedEnds(onlyExactMatches bool, maxRareOccurrences uint8) SharedEndsData
	CompareLines(aOffset, bOffset int, maxRareOccurrences uint8) (equal, approx, rare bool)
	MakeSharedEndBlockPairs(rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) (prefixPairs, suffixPairs []*BlockPair)
	MakeMiddleRangePair(rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) FileRangePair
}

type frpImpl struct {
	// Full files
	filePair FilePair

	aRange, bRange   FileRange
	aLength, bLength int

	// Lengths of the shared prefix and shared suffix of the pair of ranges.
	// A prefix (or suffix) may end with a non-rare line (e.g a blank line),
	// and thus we may want to back off to a prefix (suffix) whose last (first)
	// line is rare.
	// Note that shared prefix and shared suffix can overlap, as when comparing
	// these two strings for shared prefix and suffix: "ababababababa" and
	// "ababa".  Which of these you want depends upon context, and that context
	// is not known here.
	sharedEndsMap       map[SharedEndsKey]*SharedEndsData
}

func (p *frpImpl) BaseFilePair() FilePair { return p.filePair }

func (p *frpImpl) ARange() FileRange { return p.aRange }

func (p *frpImpl) BRange() FileRange { return p.bRange }

func (p *frpImpl) ALength() int { return p.aLength }

func (p *frpImpl) BLength() int { return p.bLength }

func (p *frpImpl) MakeSubRangePair(aOffset, aLength, bOffset, bLength int) FileRangePair {
	glog.V(1).Infof("frpImpl.MakeSubRangePair AOffsets: [%d, +%d),  BOffsets: [%d, +%d)",
		 aOffset, aLength, bOffset, bLength)
	aLo := p.aRange.ToFileIndex(aOffset)
	aHi := p.aRange.ToFileIndex(aOffset + aLength)
	bLo := p.bRange.ToFileIndex(bOffset)
	bHi := p.bRange.ToFileIndex(bOffset + bLength)
	return p.filePair.MakeSubRangePair(aLo, aHi-aLo, bLo, bHi-bLo)
}

func (p *frpImpl) BriefDebugString() string {
	var aStart, aBeyond, bStart, bBeyond int
	if p.ARange != nil {
		aStart = p.aRange.FirstIndex()
		aBeyond = p.aRange.Length() + aStart
	}
	if p.BRange != nil {
		bStart = p.bRange.FirstIndex()
		bBeyond = p.bRange.Length() + bStart
	}
	return fmt.Sprintf("FileRangePair{ARange:[%d, %d), BRange:[%d, %d)}",
		aStart, aBeyond, bStart, bBeyond)
}

func (p *frpImpl) BothAreNotEmpty() bool {
	return p.aLength > 0 && p.bLength > 0
}

func (p *frpImpl) BothAreEmpty() bool {
	return p.aLength == 0 && p.bLength == 0
}

/*
// Valid if ranges empty, or if have called MeasureSharedEnds.
func (p *frpImpl) RangesAreSame(onlyExactMatches bool) bool {
	return (p.BasicSharedEndsData.RangesAreEqual ||
		(!onlyExactMatches && p.BasicSharedEndsData.RangesAreApproximatelyEqual))
}

// Valid if ranges empty, or if have called MeasureSharedEnds.
func (p *frpImpl) RangesAreDifferent(approxIsDifferent bool) bool {
	return (p.aLength != p.bLength || (approxIsDifferent && !p.BasicSharedEndsData.RangesAreEqual) ||
		!p.BasicSharedEndsData.RangesAreApproximatelyEqual)
}
*/

func (p *frpImpl) ToFileIndices(aOffset, bOffset int) (aIndex, bIndex int) {
	aIndex = p.aRange.ToFileIndex(aOffset)
	bIndex = p.bRange.ToFileIndex(bOffset)
	return
}

func (p *frpImpl) ToRangeOffsets(aIndex, bIndex int) (aOffset, bOffset int) {
	aOffset = p.aRange.ToRangeOffset(aIndex)
	bOffset = p.bRange.ToRangeOffset(bIndex)
	return
}

// Not comparing actual content, just hashes and lengths.
func (p *frpImpl) CompareLines(aOffset, bOffset int, maxRareOccurrences uint8) (equal, approx, rare bool) {
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("frpImpl.CompareLines(%d, %d, %d) -> %v, %v, %v",
				 aOffset, bOffset, maxRareOccurrences, equal, approx, rare)
		}()
	}
	aIndex, bIndex := p.ToFileIndices(aOffset, bOffset)
	return p.filePair.CompareFileLines(aIndex, bIndex, maxRareOccurrences)
}

// Note that shared ends may overlap, as when comparing these two strings
// for shared prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *frpImpl) MeasureSharedEnds(onlyExactMatches bool, maxRareOccurrences uint8) (result SharedEndsData) {
	spewcfg := spew.Config
	spewcfg.MaxDepth = 1
	if glog.V(1) {
		glog.Infof("frpImpl.MeasureSharedEnds: onlyExactMatches=%v, maxRareOccurrences=%v",
				onlyExactMatches, maxRareOccurrences)
		defer func() {
			glog.Infof("frpImpl.MeasureSharedEnds ->\n%s", spewcfg.Sdump(result))
		}()
	}
	key := SharedEndsKey{onlyExactMatches, maxRareOccurrences}
	if p.sharedEndsMap == nil {
		p.sharedEndsMap = make(map[SharedEndsKey]*SharedEndsData)
	} else if pData, ok := p.sharedEndsMap[key]; ok {
		glog.Infof("frpImpl.MeasureSharedEnds re-using earlier measurements")
		result = *pData
		return
	}
	glog.Infof("frpImpl.MeasureSharedEnds measuring...")
	pData := &SharedEndsData{
		SharedEndsKey: key,
		Source: p,
	}
//	glog.Infof("first *pData\n%s", spewcfg.Sdump(*pData))
	p.sharedEndsMap[key] = pData
	if !p.BothAreNotEmpty() {
		if p.BothAreEmpty() {
			pData.RangesAreEqual = true
		}
		result = *pData
		return
	}
	loLength := MinInt(p.aLength, p.bLength)
	hiLength := MaxInt(p.aLength, p.bLength)
	glog.V(2).Infof("loLength=%d,  hiLength=%d", loLength, hiLength)
	allExact := true
	rareLength, nonRareLength := 0, 0
	for n := 0; n < loLength; {
		equal, approx, rare := p.CompareLines(n, n, maxRareOccurrences)
		if !(equal || (approx && !onlyExactMatches)) {
			break
		}
		n++
		nonRareLength = n
		if rare {
			rareLength = n
		}
		allExact = allExact && equal
	}
	pData.RarePrefixLength = rareLength
	pData.NonRarePrefixLength = nonRareLength
//	glog.Infof("second *pData\n%s", spewcfg.Sdump(*pData))
	if pData.NonRarePrefixLength == hiLength {
		// All lines are equal, at least after normalization. In this
		// situation, we skip spending time measuring the suffix length.
		pData.RangesAreEqual = allExact
		pData.RangesAreApproximatelyEqual = true
//		glog.Infof("third *pData\n%s", spewcfg.Sdump(*pData))
		return *pData
	}
	rareLength, nonRareLength = 0, 0
	for n := 1; n <= loLength; n++ {
		aOffset, bOffset := p.aLength-n, p.bLength-n
		equal, approx, rare := p.CompareLines(aOffset, bOffset, maxRareOccurrences)
		if !(equal || (approx && !onlyExactMatches)) {
			break
		}
		nonRareLength = n
		if rare {
			rareLength = n
		}
	}
	pData.RareSuffixLength = rareLength
	pData.NonRareSuffixLength = nonRareLength
//	glog.Infof("fourth *pData\n%s", spewcfg.Sdump(*pData))
	result = *pData
	return
}

/*
func (p *frpImpl) HasRarePrefixOrSuffix() bool {
	return (p.BasicSharedEndsData.RarePrefixLength > 0 ||
		p.BasicSharedEndsData.RareSuffixLength > 0)
}

func (p *frpImpl) HasPrefixOrSuffix() bool {
	return (p.BasicSharedEndsData.NonRarePrefixLength > 0 ||
		p.BasicSharedEndsData.NonRareSuffixLength > 0)
}

func (p *frpImpl) PrefixAndSuffixOverlap(rareEndsOnly bool) bool {
	return p.BasicSharedEndsData.PrefixAndSuffixOverlap(rareEndsOnly, p.aLength, p.bLength)
}

func (p *frpImpl) getPrefixAndSuffixLength(
	onlyExactMatches bool, maxRareOccurrences uint8, rareEndsOnly bool) (
	prefixLength, suffixLength int) {
	sharedEndsData := p.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
	return sharedEndsData.GetPrefixAndSuffixLengths(rareEndsOnly)
}
*/

func (p *frpImpl) MakeSharedEndBlockPairs(
	rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) (
	prefixPairs, suffixPairs []*BlockPair) {
	sharedEndsData := p.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
	prefixLength, suffixLength := sharedEndsData.GetPrefixAndSuffixLengths(rareEndsOnly)
	glog.Infof("frpImpl.MakeSharedEndBlockPairs: prefixLength=%d, suffixLength=%d",
	prefixLength, suffixLength)

	if prefixLength > 0 {
		aLo, bLo := p.ToFileIndices(0, 0)
		prefixPairs = append(prefixPairs, &BlockPair{
			AIndex: aLo,
			ALength: prefixLength,
			BIndex: bLo,
			BLength: prefixLength,
			IsMatch: true,
			IsNormalizedMatch: !onlyExactMatches,   // Don't know if it is an exact match or normalized.
		})
		glog.Infof("Prefix BlockPair: %v", prefixPairs[0])

	}
	if suffixLength > 0 {
		aLo, bLo := p.ToFileIndices(p.aLength - suffixLength, p.bLength - suffixLength)
		suffixPairs = append(suffixPairs, &BlockPair{
			AIndex: aLo,
			ALength: suffixLength,
			BIndex: bLo,
			BLength: suffixLength,
			IsMatch: true,
			IsNormalizedMatch: !onlyExactMatches,   // Don't know if it is an exact match or normalized.
		})
		glog.Infof("Suffix BlockPair: %v", suffixPairs[0])
	}
	// TODO Maybe provide an option to split into exact and normalized matches?
	return
}

// Returns p if there is no shared prefix or suffix.
func (p *frpImpl) MakeMiddleRangePair(
	rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) FileRangePair {
	sharedEndsData := p.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
	prefixLength, suffixLength := sharedEndsData.GetPrefixAndSuffixLengths(rareEndsOnly)
	aHi := p.aLength - suffixLength
	bHi := p.bLength - suffixLength
	return p.MakeSubRangePair(prefixLength, aHi-prefixLength, prefixLength, bHi-prefixLength)
}

//// Convert from IndexPairs of AOffset and BOffset values to AIndex and BIndex values.
//func (p *frpImpl) OffsetPairsToIndexPairs(offsetPairs []IndexPair) (indexPairs []IndexPair) {
//	for _, pair := range offsetPairs {
//		aIndex, bIndex := p.ToFileIndices(pair.Index1, pair.Index2)
//		newPair := IndexPair{aIndex, bIndex}
//		indexPairs = append(indexPairs, newPair)
//	}
//	return
//}

// Assuming here that there are no moves (relative to aRange and bRange).
func MatchingRangePairOffsetsToBlockPairs(
	frp FileRangePair, matchingOffsets []IndexPair, matchedNormalizedLines bool,
	maxRareOccurrences uint8) (blockPairs []*BlockPair) {
	matchingOffsets = append([]IndexPair(nil), matchingOffsets...)
	SortIndexPairsByIndex1(matchingOffsets)
	// Convert these to BlockPair(s) with range offsets, rather than file indices;
	// we'll switch them later in the function when it is cheaper (i.e. fewer conversions typically).
	var pair *BlockPair
	for i, m := range matchingOffsets {
		glog.V(1).Infof("matchingOffsets[%d] = %v", i, m)
		aOffset, bOffset := m.Index1, m.Index2
		isExactMatch := true
		if matchedNormalizedLines {
			isExactMatch, _, _ = frp.CompareLines(aOffset, bOffset, maxRareOccurrences)
		}
		// Can we just grow the current BlockPair?
		if pair != nil && pair.IsMatch == isExactMatch &&
			pair.ABeyond() == aOffset &&
			pair.BBeyond() == bOffset {
			// Yes, so just increase the length.
			glog.V(1).Info("Growing BlockPair")
			pair.ALength++
			pair.BLength++
			continue
		}
		// Create a new pair.
		pair = &BlockPair{
			AIndex:            aOffset,
			ALength:           1,
			BIndex:            bOffset,
			BLength:           1,
			IsMatch:           isExactMatch,
			IsNormalizedMatch: !isExactMatch,
		}
		glog.V(1).Infof("New BlockPair (range offsets, not indices): %v", pair)
		blockPairs = append(blockPairs, pair)
	}
	for _, pair := range blockPairs {
		pair.AIndex, pair.BIndex = frp.ToFileIndices(pair.AIndex, pair.BIndex)
	}
	glog.Infof("MatchingOffsetsToBlockPairs converted %d matching lines to %d BlockPairs",
		len(matchingOffsets), len(blockPairs))
	return
}
/*
func (p *frpImpl) ComputeWeightedLCSBlockPairs(
		s SimilarityFactors) (blockPairs []*BlockPair, score float32) {
	lcsOffsetPairs, score := p.ComputeWeightedLCS(&s)
	matchedNormalizedLines := s.NormalizedNonRare > 0 || s.NormalizedRare > 0
	blockPairs = p.MatchingOffsetsToBlockPairs(
		lcsOffsetPairs, matchedNormalizedLines, s.MaxRareOccurrences)
	return blockPairs, score
}
*/