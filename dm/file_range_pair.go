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
	NonRarePrefixLength, NonRareSuffixLength    int
	RarePrefixLength, RareSuffixLength          int
	RangesAreEqual, RangesAreApproximatelyEqual bool
}

type FileRangePair struct {
	// Full files
	aFile, bFile *File

	aRange, bRange   FileRange
	aLength, bLength int

	// Lengths of the shared prefix and shared suffix of the pair of ranges.
	// A prefix (or suffix) may end with a non-rare line (e.g a blank line),
	// and thus we may want to back off to a prefix (suffix) whose last (first)
	// line is rare.
	// Note that common prefix and common suffix can overlap, as when comparing
	// these two strings for common prefix and suffix: "ababababababa" and
	// "ababa".  Which of these you want depends upon context, and that context
	// is not known here.
	sharedEndsMap       map[SharedEndsKey]SharedEndsData
	basicSharedEndsData SharedEndsData
}

func MakeFullFileRangePair(aFile, bFile *File) *FileRangePair {
	return MakeFileRangePair(aFile, bFile, aFile.GetFullRange(), bFile.GetFullRange())
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
	p.basicSharedEndsData = p.MeasureSharedEnds(false, 1)
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

// Valid if ranges empty, or if have called MeasureSharedEnds.
func (p *FileRangePair) RangesAreSame(onlyExactMatches bool) bool {
	return (p.basicSharedEndsData.RangesAreEqual ||
		(!onlyExactMatches && p.basicSharedEndsData.RangesAreApproximatelyEqual))
}

// Valid if ranges empty, or if have called MeasureSharedEnds.
func (p *FileRangePair) RangesAreDifferent(approxIsDifferent bool) bool {
	return (p.aLength != p.bLength || (approxIsDifferent && !p.basicSharedEndsData.RangesAreEqual) ||
		!p.basicSharedEndsData.RangesAreApproximatelyEqual)
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

// Note that shared ends may overlap, as when comparing these two strings
// for common prefix and suffix: "ababababababa" and "ababa".
// Returns true if fully consumed.
func (p *FileRangePair) MeasureSharedEnds(onlyExactMatches bool, maxRareOccurrences uint8) SharedEndsData {
	key := SharedEndsKey{onlyExactMatches, maxRareOccurrences}
	if data, ok := p.sharedEndsMap[key]; ok {
		return data
	}
	data := SharedEndsData{
		SharedEndsKey: key,
	}
	if p.sharedEndsMap == nil {
		p.sharedEndsMap = make(map[SharedEndsKey]SharedEndsData)
	}
	if !p.BothAreNotEmpty() {
		if p.BothAreEmpty() {
			data.RangesAreEqual = true
		}
		p.sharedEndsMap[key] = data
		return data
	}
	loLength := MinInt(p.aLength, p.bLength)
	hiLength := MaxInt(p.aLength, p.bLength)
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
	data.RarePrefixLength = rareLength
	data.NonRarePrefixLength = nonRareLength
	if data.NonRarePrefixLength == hiLength {
		// All lines are equal, at least after normalization. In this
		// situation, we skip spending time measuring the suffix length.
		data.RangesAreEqual = allExact
		data.RangesAreApproximatelyEqual = true
		p.sharedEndsMap[key] = data
		return data
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
	data.RareSuffixLength = rareLength
	data.NonRareSuffixLength = nonRareLength
	p.sharedEndsMap[key] = data
	return data
}

func (p *FileRangePair) HasRarePrefixOrSuffix() bool {
	return (p.basicSharedEndsData.RarePrefixLength > 0 ||
		p.basicSharedEndsData.RareSuffixLength > 0)
}

func (p *FileRangePair) HasPrefixOrSuffix() bool {
	return (p.basicSharedEndsData.NonRarePrefixLength > 0 ||
		p.basicSharedEndsData.NonRareSuffixLength > 0)
}

func (p *FileRangePair) PrefixAndSuffixOverlap(rareEndsOnly bool) bool {
	limit, combinedLength := MinInt(p.aLength, p.bLength), 0
	if rareEndsOnly {
		combinedLength = p.basicSharedEndsData.RarePrefixLength + p.basicSharedEndsData.RareSuffixLength
	} else {
		combinedLength = p.basicSharedEndsData.NonRarePrefixLength + p.basicSharedEndsData.NonRareSuffixLength
	}
	return limit < combinedLength
}

// Returns p if there is no common prefix or suffix.
func (p *FileRangePair) MakeMiddleRangePair(rareEndsOnly bool) *FileRangePair {
	if p.PrefixAndSuffixOverlap(rareEndsOnly) {
		// Caller needs to guide the process more directly.
		cfg := spew.Config
		cfg.MaxDepth = 2
		glog.Fatalf("MakeMiddleRangePair(%v) prefix and suffix overlap; FileRangePair:\n%s",
			cfg.Sdump(p))
	}

	var lo, suffixLength int

	if rareEndsOnly {
		lo = p.basicSharedEndsData.RarePrefixLength
		suffixLength = p.basicSharedEndsData.RareSuffixLength
	} else {
		lo = p.basicSharedEndsData.NonRarePrefixLength
		suffixLength = p.basicSharedEndsData.NonRareSuffixLength
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
