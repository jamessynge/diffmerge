package dm

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

var TO_BE_DELETED = glog.CopyStandardLogTo

// Aim here is to be able to represent two files, or two ranges, one in each
// of two files, as a single object.

type FileRangePair struct {
	// Full files
	aFile, bFile *File

	aRange, bRange                              FileRange
	aLength, bLength                            int
	rangesAreEqual, rangesAreApproximatelyEqual bool

	// Lengths of the common (shared) prefix and suffix of the pair of ranges.
	// A prefix (or suffix) may end with a non-rare line (e.g a blank line),
	// and thus we may want to back off to a prefix (suffix) whose last (first)
	// line is rare.
	// Note that common prefix and common suffix can overlap, as when comparing
	// these two strings for common prefix and suffix: "ababababababa" and
	// "ababa".  Which of these you want depends upon context, which is not
	// known here.
	commonPrefixLength, commonSuffixLength int
	rarePrefixLength, rareSuffixLength     int
	haveMeasuredCommonEnds                 bool
}

func MakeFileRangePair(aFile, bFile *File, aRange, bRange FileRange) *FileRangePair {
	p := &FileRangePair{
		aFile:  aFile,
		bFile:  bFile,
		aRange: aRange,
		bRange: bRange,
	}
	if !FileRangeIsEmpty(aRange) {
		p.aLength = aRange.LineCount()
	}
	if !FileRangeIsEmpty(bRange) {
		p.bLength = bRange.LineCount()
	}
	if p.aLength == 0 && p.bLength == 0 {
		p.rangesAreEqual = true
		p.rangesAreApproximatelyEqual = true
		p.haveMeasuredCommonEnds = true
	}
	return p
}

func (p *FileRangePair) MakeSubRangePair(aOffset, aLength, bOffset, bLength int) *FileRangePair {
	aLo := p.aRange.ToFileIndex(aOffset)
	aHi := p.aRange.ToFileIndex(aOffset + aLength)
	bLo := p.bRange.ToFileIndex(bOffset)
	bHi := p.bRange.ToFileIndex(bOffset + bLength)
	aRange := p.aFile.MakeSubRange(aLo, aHi-aLo)
	bRange := p.bFile.MakeSubRange(bLo, bHi-bLo)
	pair := MakeFileRangePair(p.aFile, p.bFile, aRange, bRange)
	return pair
}

func (p *FileRangePair) BriefDebugString() string {
	var aStart, aBeyond, bStart, bBeyond int
	if p.aRange != nil {
		aStart = p.aRange.FirstIndex()
		aBeyond = p.aRange.LineCount() + aStart
	}
	if p.bRange != nil {
		bStart = p.bRange.FirstIndex()
		bBeyond = p.bRange.LineCount() + bStart
	}
	return fmt.Sprintf("FileRangePair{aRange:[%d, %d), bRange:[%d, %d)}",
		aStart, aBeyond, bStart, bBeyond)
}

func (p *FileRangePair) BothAreNotEmpty() bool {
	return p.aLength > 0 && p.bLength > 0
}

func (p *FileRangePair) BothAreEmpty() bool {
	return p.aLength == 0 && p.bLength == 0
}

// Only valid p.haveMeasuredCommonEnds is true.
func (p *FileRangePair) RangesAreSame(onlyExactMatches bool) bool {
	return p.rangesAreEqual || (!onlyExactMatches && p.rangesAreApproximatelyEqual)
}

// Only valid p.haveMeasuredCommonEnds is true.
func (p *FileRangePair) RangesAreDifferent(approxIsDifferent bool) bool {
	return (p.aLength != p.bLength || (approxIsDifferent && !p.rangesAreEqual) ||
		!p.rangesAreApproximatelyEqual)
}

func (p *FileRangePair) ToFileIndices(aOffset, bOffset int) (aIndex, bIndex int) {
	aIndex = p.aRange.ToFileIndex(aOffset)
	bIndex = p.bRange.ToFileIndex(bOffset)
	return
}

func (p *FileRangePair) ToRangeOffsets(aIndex, bIndex int) (aOffset, bOffset int) {
	aOffset = p.aRange.ToRangeOffset(aIndex)
	bOffset = p.bRange.ToRangeOffset(bIndex)
	return
}

// Not comparing actual content, just hashes and lengths.
func (p *FileRangePair) CompareLines(aOffset, bOffset int, maxRareOccurrences uint8) (equal, approx, rare bool) {
	aIndex, bIndex := p.ToFileIndices(aOffset, bOffset)
	aLP := &p.aFile.Lines[aIndex]
	bLP := &p.bFile.Lines[bIndex]
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

// Note that common prefix may overlap, as when comparing these two strings
// for common prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *FileRangePair) MeasureCommonEnds(onlyExactMatches bool, maxRareOccurrences uint8) (rangesSame bool) {
	if p.haveMeasuredCommonEnds {
		return p.rangesAreEqual || (!onlyExactMatches && p.rangesAreApproximatelyEqual)
	}
	p.haveMeasuredCommonEnds = true
	if !p.BothAreNotEmpty() {
		p.rangesAreEqual = p.BothAreEmpty()
		return p.rangesAreEqual
	}
	limit := MinInt(p.aLength, p.bLength)
	allExact := true
	rareLength, nonRareLength := 0, 0
	for n := 0; n < limit; {
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
	p.rarePrefixLength = rareLength
	p.commonPrefixLength = nonRareLength

	if p.commonPrefixLength == p.aLength && p.commonPrefixLength == p.bLength {
		// In this situation, we skip spending time measuring the suffix length.
		p.rangesAreEqual = allExact
		p.rangesAreApproximatelyEqual = true
		return true
	}

	rareLength, nonRareLength = 0, 0
	for n := 1; n <= limit; n++ {
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
	p.rareSuffixLength = rareLength
	p.commonSuffixLength = nonRareLength
	return
}

func (p *FileRangePair) HasRarePrefixOrSuffix() bool {
	return p.rarePrefixLength > 0 || p.rarePrefixLength > 0
}

func (p *FileRangePair) HasPrefixOrSuffix() bool {
	return p.commonPrefixLength > 0 || p.commonPrefixLength > 0
}

func (p *FileRangePair) PrefixAndSuffixOverlap(rareEndsOnly bool) bool {
	limit, combinedLength := MinInt(p.aLength, p.bLength), 0
	if rareEndsOnly {
		combinedLength = p.rarePrefixLength + p.rareSuffixLength
	} else {
		combinedLength = p.commonPrefixLength + p.commonSuffixLength
	}
	return limit < combinedLength
}

// Returns p if there is no common prefix or suffix.
func (p *FileRangePair) MakeMiddleRangePair(rareEndsOnly bool) *FileRangePair {
	if !p.haveMeasuredCommonEnds {
		glog.Fatalf("Middle range has not be measured for %s", p.BriefDebugString())
	}
	if p.PrefixAndSuffixOverlap(rareEndsOnly) {
		// Caller needs to guide the process more directly.
		cfg := spew.Config
		cfg.MaxDepth = 2
		glog.Fatalf("MakeMiddleRangePair(%v) prefix and suffix overlap; FileRangePair:\n%s",
			cfg.Sdump(p))
	}

	var lo, suffixLength int

	if rareEndsOnly {
		lo = p.rarePrefixLength
		suffixLength = p.rareSuffixLength
	} else {
		lo = p.commonPrefixLength
		suffixLength = p.commonSuffixLength
	}
	if lo == 0 && suffixLength == 0 {
		return p
	}
	aHi := p.aLength - suffixLength
	bHi := p.bLength - suffixLength

	return p.MakeSubRangePair(lo, aHi-lo, lo, bHi-lo)
}

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

func (p *FileRangePair) ComputeWeightedLCS(s *SimilarityFactors) (lcsOffsetPairs []IndexPair, score float32) {
	computeSimilarity := func(aOffset, bOffset int) float32 {
		return s.SimilarityOfLines(p, aOffset, bOffset)
	}
	if p.aLength > 0 && p.bLength > 0 {
		glog.Infof("ComputeWeightedLCS: %s", p.BriefDebugString())
		lcsOffsetPairs, score = WeightedLCS(p.aLength, p.bLength, computeSimilarity)
		glog.Infof("ComputeWeightedLCS: LCS length == %d", len(lcsOffsetPairs))
	}
	return
}

// Convert from IndexPairs of AOffset and BOffset values to AIndex and BIndex values.
func (p *FileRangePair) OffsetPairsToIndexPairs(offsetPairs []IndexPair) (indexPairs []IndexPair) {
	for _, pair := range offsetPairs {
		aIndex, bIndex := p.ToFileIndices(pair.Index1, pair.Index2)
		newPair := IndexPair{aIndex, bIndex}
		indexPairs = append(indexPairs, newPair)
	}
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
