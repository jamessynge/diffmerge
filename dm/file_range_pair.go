package dm

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

var TO_BE_DELETED = glog.CopyStandardLogTo

// Aim here is to be able to represent two files, or two ranges, one in each
// of two files, as a single object.

type SharedEndsKey struct {
	OnlyExactMatches   bool
	MaxRareOccurrences uint8
}

type SharedEndsData struct {
	SharedEndsKey
	// If the lines in the range are equal, or equal after normalization
	// (approximately equal), then one or both of these booleans are true,
	// and the prefix and suffix lengths are 0.
	RangesAreEqual, RangesAreApproximatelyEqual bool
	
	// The FileRangePair which was measured to produce this.
	Source *FileRangePair

	NonRarePrefixLength, NonRareSuffixLength    int
	RarePrefixLength, RareSuffixLength          int
}

func (p *SharedEndsData) PrefixAndSuffixOverlap(rareEndsOnly bool) {
	limit, combinedLength := MinInt(p.Source.ALength, p.Source.BLength), 0
	if rareEndsOnly {
		combinedLength = p.RarePrefixLength + p.RareSuffixLength
	} else {
		combinedLength = p.NonRarePrefixLength + p.NonRareSuffixLength
	}
	return limit < combinedLength
}

func (p *SharedEndsData) HasRarePrefixOrSuffix() bool {
	return p.RarePrefixLength > 0 || p.RareSuffixLength > 0
}

func (p *SharedEndsData) HasPrefixOrSuffix() bool {
	return p.NonRarePrefixLength > 0 || p.NonRareSuffixLength > 0
}

func (p *SharedEndsData) GetPrefixAndSuffixLengths(rareEndsOnly bool) {
	if p.PrefixAndSuffixOverlap(rareEndsOnly) {
		// Caller needs to guide the process more directly.
		cfg := spew.Config
		cfg.MaxDepth = 2
		glog.Fatal("GetPrefixAndSuffixLengths: prefix and suffix overlap;\n",
							 cfg.Sdump(p))
	}
	if rareEndsOnly {
		return p.RarePrefixLength, p.RareSuffixLength
	} else {
		return p.NonRarePrefixLength, p.NonRareSuffixLength
	}
	return
}

////////////////////////////////////////////////////////////////////////////////

type FileRangePair struct {
	// Full files
	AFile, BFile *File

	ARange, BRange   FileRange
	ALength, BLength int

	// Lengths of the shared prefix and shared suffix of the pair of ranges.
	// A prefix (or suffix) may end with a non-rare line (e.g a blank line),
	// and thus we may want to back off to a prefix (suffix) whose last (first)
	// line is rare.
	// Note that shared prefix and shared suffix can overlap, as when comparing
	// these two strings for shared prefix and suffix: "ababababababa" and
	// "ababa".  Which of these you want depends upon context, and that context
	// is not known here.
	SharedEndsMap       map[SharedEndsKey]*SharedEndsData
}

func MakeFullFileRangePair(aFile, bFile *File) *FileRangePair {
	return MakeFileRangePair(aFile, bFile, aFile.GetFullRange(), bFile.GetFullRange())
}

func MakeFileRangePair(aFile, bFile *File, aRange, bRange FileRange) *FileRangePair {
	p := &FileRangePair{
		AFile:  aFile,
		BFile:  bFile,
		ARange: aRange,
		BRange: bRange,
	}
	if !FileRangeIsEmpty(aRange) {
		p.ALength = aRange.LineCount()
	}
	if !FileRangeIsEmpty(bRange) {
		p.BLength = bRange.LineCount()
	}
	return p
}

func (p *FileRangePair) MakeSubRangePair(aOffset, aLength, bOffset, bLength int) *FileRangePair {
	aLo := p.ARange.ToFileIndex(aOffset)
	aHi := p.ARange.ToFileIndex(aOffset + aLength)
	bLo := p.BRange.ToFileIndex(bOffset)
	bHi := p.BRange.ToFileIndex(bOffset + bLength)
	aRange := p.AFile.MakeSubRange(aLo, aHi-aLo)
	bRange := p.BFile.MakeSubRange(bLo, bHi-bLo)
	pair := MakeFileRangePair(p.AFile, p.BFile, aRange, bRange)
	return pair
}

func (p *FileRangePair) BriefDebugString() string {
	var aStart, aBeyond, bStart, bBeyond int
	if p.ARange != nil {
		aStart = p.ARange.FirstIndex()
		aBeyond = p.ARange.LineCount() + aStart
	}
	if p.BRange != nil {
		bStart = p.BRange.FirstIndex()
		bBeyond = p.BRange.LineCount() + bStart
	}
	return fmt.Sprintf("FileRangePair{BRange:[%d, %d), BRange:[%d, %d)}",
		aStart, aBeyond, bStart, bBeyond)
}

func (p *FileRangePair) BothAreNotEmpty() bool {
	return p.ALength > 0 && p.BLength > 0
}

func (p *FileRangePair) BothAreEmpty() bool {
	return p.ALength == 0 && p.BLength == 0
}

/*
// Valid if ranges empty, or if have called MeasureSharedEnds.
func (p *FileRangePair) RangesAreSame(onlyExactMatches bool) bool {
	return (p.BasicSharedEndsData.RangesAreEqual ||
		(!onlyExactMatches && p.BasicSharedEndsData.RangesAreApproximatelyEqual))
}

// Valid if ranges empty, or if have called MeasureSharedEnds.
func (p *FileRangePair) RangesAreDifferent(approxIsDifferent bool) bool {
	return (p.ALength != p.BLength || (approxIsDifferent && !p.BasicSharedEndsData.RangesAreEqual) ||
		!p.BasicSharedEndsData.RangesAreApproximatelyEqual)
}
*/

func (p *FileRangePair) ToFileIndices(aOffset, bOffset int) (aIndex, bIndex int) {
	aIndex = p.ARange.ToFileIndex(aOffset)
	bIndex = p.BRange.ToFileIndex(bOffset)
	return
}

func (p *FileRangePair) ToRangeOffsets(aIndex, bIndex int) (aOffset, bOffset int) {
	aOffset = p.ARange.ToRangeOffset(aIndex)
	bOffset = p.BRange.ToRangeOffset(bIndex)
	return
}

// Not comparing actual content, just hashes and lengths.
func (p *FileRangePair) CompareLines(aOffset, bOffset int, maxRareOccurrences uint8) (equal, approx, rare bool) {
	aIndex, bIndex := p.ToFileIndices(aOffset, bOffset)
	aLP := &p.AFile.Lines[aIndex]
	bLP := &p.BFile.Lines[bIndex]
	if aLP.NormalizedHash != bLP.NormalizedHash {
		return
	}
	if aLP.NormalizedLength != bLP.NormalizedLength {
		return
	}
	approx = true
	if aLP.Hash == bLP.Hash && aLP.Length == bLP.Length {
		equal = true
	}
	if aLP.ProbablyCommon || bLP.ProbablyCommon {
		return
	}
	if maxRareOccurrences < aLP.CountInFile {
		return
	}
	if maxRareOccurrences < bLP.CountInFile {
		return
	}
	rare = true
	return
}

// Note that shared ends may overlap, as when comparing these two strings
// for shared prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *FileRangePair) MeasureSharedEnds(onlyExactMatches bool, maxRareOccurrences uint8) SharedEndsData {
	key := SharedEndsKey{onlyExactMatches, maxRareOccurrences}
	if p.sharedEndsMap == nil {
		p.sharedEndsMap = make(map[SharedEndsKey]SharedEndsData)
	} else if pData, ok := p.sharedEndsMap[key]; ok {
		return *pData
	}
	pData := &SharedEndsData{
		SharedEndsKey: key,
		Source: p,
	}
	p.sharedEndsMap[key] = pData
	if !p.BothAreNotEmpty() {
		if p.BothAreEmpty() {
			pData.RangesAreEqual = true
		}
		return *pData
	}
	loLength := MinInt(p.ALength, p.BLength)
	hiLength := MaxInt(p.ALength, p.BLength)
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
	if pData.NonRarePrefixLength == hiLength {
		// All lines are equal, at least after normalization. In this
		// situation, we skip spending time measuring the suffix length.
		pData.RangesAreEqual = allExact
		pData.RangesAreApproximatelyEqual = true
		return *pData
	}
	rareLength, nonRareLength = 0, 0
	for n := 1; n <= loLength; n++ {
		aOffset, bOffset := p.ALength-n, p.BLength-n
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
	return *pData
}

/*
func (p *FileRangePair) HasRarePrefixOrSuffix() bool {
	return (p.BasicSharedEndsData.RarePrefixLength > 0 ||
		p.BasicSharedEndsData.RareSuffixLength > 0)
}

func (p *FileRangePair) HasPrefixOrSuffix() bool {
	return (p.BasicSharedEndsData.NonRarePrefixLength > 0 ||
		p.BasicSharedEndsData.NonRareSuffixLength > 0)
}

func (p *FileRangePair) PrefixAndSuffixOverlap(rareEndsOnly bool) bool {
	return p.BasicSharedEndsData.PrefixAndSuffixOverlap(rareEndsOnly, p.ALength, p.BLength)
}

func (p *FileRangePair) getPrefixAndSuffixLength(
	onlyExactMatches bool, maxRareOccurrences uint8, rareEndsOnly bool) (
	prefixLength, suffixLength int) {
	sharedEndsData := p.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
	return sharedEndsData.GetPrefixAndSuffixLengths(rareEndsOnly)
}
*/

func (p *FileRangePair) MakeSharedEndBlockPairs(
	rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) (
	prefixPairs, suffixPairs []*BlockPair) {
	sharedEndsData := p.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
	prefixLength, suffixLength := sharedEndsData.GetPrefixAndSuffixLengths(rareEndsOnly)
	if prefixLength > 0 {
		aLo, bLo := p.ToFileIndices(0, 0)
		aHi, bHi := p.ToFileIndices(prefixLength, prefixLength)
		prefixPairs = append(prefixPairs, &BlockPair{
			AIndex: aLo,
			ALength: prefixLength,
			BIndex: bLo,
			BLength: prefixLength,
			IsMatch: true,
			IsNormalizedMatch: !onlyExactMatches,   // Don't know if it is an exact match or normalized.
		})
	}
	if suffixLength > 0 {
		aLo, bLo := p.ToFileIndices(p.ALength - suffixLength, p.BLength - suffixLength)
		aHi, bHi := p.ToFileIndices(p.ALength, p.BLength)
		suffixPairs = append(suffixPairs, &BlockPair{
			AIndex: aLo,
			ALength: prefixLength,
			BIndex: bLo,
			BLength: prefixLength,
			IsMatch: true,
			IsNormalizedMatch: !onlyExactMatches,   // Don't know if it is an exact match or normalized.
		})
	}
	// TODO Maybe provide an option to split into exact and normalized matches?
	return
}

// Returns p if there is no shared prefix or suffix.
func (p *FileRangePair) MakeMiddleRangePair(
	rareEndsOnly, onlyExactMatches bool, maxRareOccurrences uint8) *FileRangePair {
	sharedEndsData := p.MeasureSharedEnds(onlyExactMatches, maxRareOccurrences)
	prefixLength, suffixLength := sharedEndsData.GetPrefixAndSuffixLengths(rareEndsOnly)
	aHi := p.ALength - suffixLength
	bHi := p.BLength - suffixLength
	return p.MakeSubRangePair(prefixLength, aHi-prefixLength, prefixLength, bHi-prefixLength)
}

//// Convert from IndexPairs of AOffset and BOffset values to AIndex and BIndex values.
//func (p *FileRangePair) OffsetPairsToIndexPairs(offsetPairs []IndexPair) (indexPairs []IndexPair) {
//	for _, pair := range offsetPairs {
//		aIndex, bIndex := p.ToFileIndices(pair.Index1, pair.Index2)
//		newPair := IndexPair{aIndex, bIndex}
//		indexPairs = append(indexPairs, newPair)
//	}
//	return
//}

type SimilarityFactors struct {
	ExactRare          float32
	NormalizedRare     float32
	ExactNonRare       float32
	NormalizedNonRare  float32
	MaxRareOccurrences uint8
}

func (s *SimilarityFactors) SimilarityOfLines(pair *FileRangePair, aOffset, bOffset int) float32 {
	equal, approx, rare := pair.CompareLines(aOffset, bOffset, s.MaxRareOccurrences)
	if equal {
		if rare {
			return s.ExactRare
		} else {
			return s.ExactNonRare
		}
	}
	if approx {
		if rare {
			return s.NormalizedRare
		} else {
			return s.NormalizedNonRare
		}
	}
	return 0
}

func (p *FileRangePair) ComputeWeightedLCS(
	s *SimilarityFactors) (lcsOffsetPairs []IndexPair, score float32) {
	if p.ALength == 0 || p.BLength == 0 { return }
	computeSimilarity := func(aOffset, bOffset int) float32 {
		return s.SimilarityOfLines(p, aOffset, bOffset)
	}
	glog.Infof("ComputeWeightedLCS: %s", p.BriefDebugString())
	lcsOffsetPairs, score = WeightedLCS(p.ALength, p.BLength, computeSimilarity)
	glog.Infof("ComputeWeightedLCS: LCS length == %d,   LCS score: %v", len(lcsOffsetPairs), score)
	return
}

// Assuming here that there are no moves (relative to aRange and bRange).
func (p *FileRangePair) MatchingOffsetsToBlockPairs(
	matchingOffsets []IndexPair, matchedNormalizedLines bool,
	maxRareOccurrences uint8) (blockPairs []*BlockPair) {
	matchingOffsets = append([]IndexPair(nil), matchingOffsets...)
	SortIndexPairsByIndex1(matchingOffsets)
	// Convert these to BlockPair(s) with range offsets, rather than file indices;
	// we'll switch them later when it is cheaper (i.e. fewer conversions typically).
	var pair *BlockPair
	for i, m := range matchingOffsets {
		glog.V(1).Infof("matchingOffsets[%d] = %v", i, m)
		aOffset, bOffset := m.Index1, m.Index2
		isExactMatch := true
		if matchedNormalizedLines {
			isExactMatch, _, _ = p.CompareLines(aOffset, bOffset, maxRareOccurrences)
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
		pair.AIndex, pair.BIndex = p.ToFileIndices(pair.AIndex, pair.BIndex)
	}
	glog.Infof("MatchingOffsetsToBlockPairs converted %d matching lines to %d BlockPairs",
		len(matchingOffsets), len(blockPairs))
	return
}

func (p *FileRangePair) ComputeWeightedLCSBlockPairs(
		s SimilarityFactors) (blockPairs []*BlockPair, score float32) {
	lcsOffsetPairs, score := p.ComputeWeightedLCS(&s)
	matchedNormalizedLines := s.NormalizedNonRare > 0 || s.NormalizedRare > 0
	blockPairs = p.MatchingOffsetsToBlockPairs(
		lcsOffsetPairs, matchedNormalizedLines, s.MaxRareOccurrences)
	return blockPairs, score
}

func (p *FileRangePair) CanFillGapWithMatches(pair1, pair2 *BlockPair) bool {
	// File indices
	aLo, bLo := pair1.ABeyond(), pair1.BBeyond()
	aHi, bHi := pair2.AIndex, pair2.BIndex
	// C
	

}