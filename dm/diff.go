package dm

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

// We assume that aFile is the base file version and that bFile is derived from
// aFile. Therefore we also assume that block copies occur in bFile, but not
// in aFile.  As a result, when processing gaps (unmatched regions), we focus
// on bFile when finding gaps, and then find corresponding regions in aFile,
// which may be processed with more than one section of bFile.

// Note that this is not intended to be efficient, just effective (hopefully).
// If it turns out to be useful, but slow, there is plenty of room for optimization.

// TODO How can we use sentinal lines here? Maybe produce a new FileRange
// that has unique sentinals at the start and end of each Full FileRange;
// or common start sentinals at the start of every Full FileRange, and
// common end sentinals at the end of every Full FileRange, so that we'd be
// guaranteed of a match at the start, and then not worry about gap filling
// before BOF or after EOF.

// TODO Introduce two ordered data structures for storing the *BlockPair's,
// one sorting by AIndex, the other by BIndex. This will eliminate all the
// SortBlockPairsBy*Index calls.
// We'll need the ability to:
// * insert elements, perhaps with a hint (e.g. when splitting a BlockPair,
//	 we might want to replace one entry with several);
// * forward iterate over the members, tolerating modifications (e.g. filling
//	 gaps while iterating, or splitting an entry);
// * get the last member, and possibly the first;
// * lookup members (e.g. when filling gaps by AIndex, we'll need to find
//	 the neighbors of the BlockPair in B).
// We might be able to start with a single BlockPair representing a mismatch
// between the entirety of the two files, then we'd try to create matches within
// that BlockPair. For easy of operations we might want to insert two other
// BlockPairs (sentinals) marking the start and end of the two files.

func PerformDiff(aFile, bFile *File, config DifferencerConfig) (pairs []*BlockPair) {
	p := diffState{
		aFile:      aFile,
		bFile:      bFile,
		aFullRange: aFile.GetFullRange(),
		bFullRange: bFile.GetFullRange(),
		aRange:     aFile.GetFullRange(),
		bRange:     bFile.GetFullRange(),
		//		aRemainingCount: aFile.GetLineCount(),
		bRemainingCount: bFile.GetLineCount(),
		config:          config,
	}

	glog.Info("PerformDiff entry, diffState:\n", p.SDumpToDepth(1))

	if true {
		// Experiment in an effort to better handle moves followed by edits, and
		// also copies (possibly followed by edits):
		// 1) Match common prefix and suffix.
		// 2) Align rare lines with LCS.
		// 3) Extend common prefix and suffix of the LCS entries, with no overlap
		//    (i.e. so far there can be no copies, so there can only be 1-to-1
		//    matches between A and B lines, or no match at all).
		// 4) If there are long-ish insertions (not changes) in B remaining,
		//		attempt to match those one at a time with the entirety of A. If we
		//    get a good match, then we can create BlockPair(s) for the match,
		//    multiple if there are both matches (exacted and/or normalized) and
		//    changes. An advantage of this approach is that we can readily
		//    identify where there are moves and where there are copies.
		// 5) TODO in a future experiment: if there are changes where there are
		//    significantly more lines in B than A, we can do sub-line LCS to see
		//    if we can identify where there might be an insertion among the
		//    B lines, rather than just edits, and then attempt to match just that
		//    portion with A.

		p.exp_phase1_ends()

		p.exp_phase2_and_3_lcs()

	} else {

		if p.config.alignRareLines {
			p.processOneRangePair()
			glog.Info("PerformDiff processed rare lines, diffState:\n", p.SDumpToDepth(1))
			p.config.alignRareLines = false
		}
		p.processAllGapsInB(true, func() { p.processOneRangePair() })
		glog.Info("PerformDiff processed gaps in B, diffState:\n", p.SDumpToDepth(1))

	}

	p.splitMixedMatches()
	glog.Info("PerformDiff split mixed matches, diffState:\n", p.SDumpToDepth(1))

	p.fillAllGaps()
	glog.Info("PerformDiff filled gaps, diffState:\n", p.SDumpToDepth(1))

	if p.bRemainingCount > 0 {
		panic("why are there remaining b gaps")
	}

	// Combine adjacent BlockPairs of the same type.
	p.sortPairsByA()
	p.pairs = CombineBlockPairs(p.pairs)
	p.sortPairsByB()
	p.pairs = CombineBlockPairs(p.pairs)

	output := p.pairs

	if glog.V(1) {
		glog.Info("PerformDiff exit")
		for n, pair := range output {
			glog.Infof("PerformDiff output[%d] = %v", n, pair)
		}
	}

	return output
}

type diffState struct {
	// Full files
	aFile, bFile           *File
	aFullRange, bFullRange FileRange

	// Range being considered by matchRangeEnds (to match common prefix and suffix
	// lines), matchRangeMiddle (to find matches with possible mismatches
	// between them). Set by extendAllMatches and fillAllGaps.
	aRange, bRange FileRange

	// Discovered pairs (matches, moves, copies, mismatches).
	pairs []*BlockPair

	// Sort order, if known, of pairs
	isSortedByA, isSortedByB bool

	// Set when filling gaps.
	pairsByA, pairsByB       []*BlockPair
	pair2AOrder, pair2BOrder map[*BlockPair]int

	// Number of lines in B not yet represented in pairs. Not counting A lines
	// because each line in A may be matched with zero or more B lines.
	bRemainingCount int

	// Have we detected that a move (or copy) exists? Don't have a means of
	// detecting just copies (yet).
	detectedAMove bool

	// Controls operation.
	config DifferencerConfig
}

// Do exact and approximate matching at the start and end of the full ranges.
func (p *diffState) exp_phase1_ends() {
	p.assertNoPairs()

	if !FileRangeIsEmpty(p.aRange) && !FileRangeIsEmpty(p.bRange) {
		if p.matchRangeEnds( /*prefix*/ true /*suffix*/, true /*normalize*/, false) {
			return
		}
		if p.config.matchNormalizedEnds &&
			p.matchRangeEnds( /*prefix*/ true /*suffix*/, true /*normalize*/, true) {
			return
		}
	}
}

// Match the middle (not the common ends) using LCS, then extend.
func (p *diffState) exp_phase2_and_3_lcs() {
	var cfg DifferencerConfig = p.config
	defer func() {
		p.config = cfg
	}()
	p.config.detectBlockMoves = false
	p.config.matchEnds = true
	p.config.matchNormalizedEnds = p.config.alignNormalizedLines

	p.matchRangeMiddle()

	if p.config.alignRareLines {
		p.config.alignRareLines = false
		p.processAllGapsInB(true, func() { p.processOneRangePair() })
	}
}

func (p *diffState) exp_phase5_moves_and_copies() {
	var cfg DifferencerConfig = p.config
	defer func() {
		p.config = cfg
	}()
	p.config.detectBlockMoves = false
	p.config.matchEnds = false

	var newPairs []*BlockPair

	minExtraBLinesForProcessing := 5

	p.processAllGapsInB(true, func() {
		extraBLines := p.bRange.GetLineCount() - p.aRange.GetLineCount()
		if extraBLines < minExtraBLinesForProcessing {
			return
		}
		// Match p.bRange to all of A.
		p.aRange = p.aFullRange
		somePairs := p.rangeToBlockPairs()
		if len(somePairs) == 0 {
			return
		}

		// TODO Decide if these pairs are worth accepting; i.e. several matches of
		// empty lines in various parts of A aren't useful; we're looking for a
		// tight match, showing that a move or copy has occurred.

		glog.V(1).Info("exp_phase5_moves_and_copies found ", len(newPairs),
			" BlockPairs while matching a gap with ", extraBLines, " extra lines in B")
	})
}

func (p *diffState) processOneRangePair() {
	if !FileRangeIsEmpty(p.aRange) && !FileRangeIsEmpty(p.bRange) {
		if p.config.matchEnds {
			if p.matchRangeEnds( /*prefix*/ true /*suffix*/, true /*normalize*/, false) {
				return
			}
			if p.config.matchNormalizedEnds &&
				p.matchRangeEnds( /*prefix*/ true /*suffix*/, true /*normalize*/, true) {
				return
			}
		}
	}

	if !FileRangeIsEmpty(p.aRange) && !FileRangeIsEmpty(p.bRange) {
		// Figure out an alignment of the remains after prefix and suffix matching.
		// Note that if p.config.alignRareLines==true, then there may be matches
		// in the range that we've not yet made, but can later.
		p.matchRangeMiddle()
	}
}

func (p *diffState) SDumpToDepth(depth int) string {
	var cs spew.ConfigState = spew.Config
	cs.MaxDepth = depth
	return cs.Sdump(p)
}

func (p *diffState) sortPairsByA() {
	if !p.isSortedByA {
		SortBlockPairsByAIndex(p.pairs)
		p.isSortedByA = true
		p.isSortedByB = false // Might be, but can't be sure.
	}
}

func (p *diffState) sortPairsByB() {
	if !p.isSortedByB {
		SortBlockPairsByBIndex(p.pairs)
		p.isSortedByA = false // Might be, but can't be sure.
		p.isSortedByB = true
	}
}

func (p *diffState) addBlockPair(bp *BlockPair) {
	// TODO Add optional checking for validity: correct line indices, and not
	// overlapping existing BlockPairs. For now, just make sure we don't have
	// fewer than zero lines remaining.
	if bp != nil {
		glog.Infof("addBlockPair: bp:\n%s", spew.Sdump(bp))
		if len(p.pairs) > 0 {
			lastPair := p.pairs[len(p.pairs)-1]
			if p.isSortedByA && bp.AIndex < lastPair.AIndex+lastPair.ALength {
				p.isSortedByA = false
			}
			if p.isSortedByB && bp.BIndex < lastPair.BIndex+lastPair.BLength {
				p.isSortedByB = false
			}
		} else {
			p.isSortedByA = true
			p.isSortedByB = true
		}
		p.pairs = append(p.pairs, bp)
		if bp.IsMove {
			p.detectedAMove = true
		}
		p.bRemainingCount -= bp.BLength
		if p.bRemainingCount < 0 {
			glog.Fatalf("Adding BlockPair dropped remaining count below zero!\n"+
				"BlockPair: %v\ndiffState: %v", *bp, *p)
		}
		//		glog.Infof("addBlockPair: aRemainingCount=%d	 bRemainingCount=%d",
		//		p.aRemainingCount, p.bRemainingCount)
		glog.Infof("addBlockPair: bRemainingCount=%d",
			p.bRemainingCount)
	}
}

// Given the current state of p.aRange and p.bRange, attempt to shrink the
// ranges by matching their prefixes and suffix. Returns true if one or the
// other of the ranges is empty either before or after matching.
func (p *diffState) matchRangeEnds(prefix, suffix, normalized bool) (done bool) {
	var newPairs []*BlockPair
	p.aRange, p.bRange, newPairs = MatchCommonEnds(
		p.aRange, p.bRange, prefix, suffix, normalized)
	for _, pair := range newPairs {
		glog.V(1).Info("matchRangeEnds adding BlockPair")
		p.addBlockPair(pair)
	}
	return FileRangeIsEmpty(p.aRange) || FileRangeIsEmpty(p.bRange)
}

func (p *diffState) matchWithMoves(
	aLines, bLines []LinePos,
	getHash func(lp LinePos) uint32) []BlockMatch {
	glog.V(1).Info("matchWithMoves")

	matches := BasicTichyMaximalBlockMoves(aLines, bLines, getHash)
	return matches
}

func (p *diffState) linearMatch(
	aLines, bLines []LinePos, normalize bool) []BlockMatch {
	glog.V(1).Info("linearMatch")
	var getSimilarity func(aIndex, bIndex int) float32
	var normalizedSimilarity float32
	if normalize {
		normalizedSimilarity = float32(p.config.lcsNormalizedSimilarity)
		if !(0 < normalizedSimilarity && normalizedSimilarity <= 1) {
			glog.Fatalf("--lcs-normalized-similarity=%v is out of range (0,1].",
				normalizedSimilarity)
		}
		getSimilarity = func(aIndex, bIndex int) float32 {
			if aLines[aIndex].Hash == bLines[bIndex].Hash {
				return 1
			} else if aLines[aIndex].NormalizedHash == bLines[bIndex].NormalizedHash {
				return normalizedSimilarity
			} else {
				return 0
			}
		}
	} else {
		getSimilarity = func(aIndex, bIndex int) float32 {
			if aLines[aIndex].Hash == bLines[bIndex].Hash {
				return 1
			} else {
				return 0
			}
		}
	}

	abIndices := WeightedLCS(len(aLines), len(bLines), getSimilarity)
	var result []BlockMatch
	for _, ab := range abIndices {
		result = append(result, BlockMatch{
			AIndex: ab.Index1, // Indices in aLines and bLines, respectively.
			BIndex: ab.Index2,
			Length: 1,
		})
	}
	return result
}

func (p *diffState) convertMatchesToBlockPairs(
	aLines, bLines []LinePos, normalize bool, matches []BlockMatch) (
	newPairs []*BlockPair) {
	// Convert these to BlockPairs. Since they aren't necessarily contiguous,
	// and there might be moves, we'll process them line by line, building
	// BlockPairs that are contiguous.

	convertIndices := func(ai, bi int) (aIndex, bIndex int) {
		return aLines[ai].Index, bLines[bi].Index
	}

	SortBlockMatchesByBIndex(matches)
	var prevPair *BlockPair
	for i, m := range matches {
		glog.V(1).Infof("matches[%d] = %v", i, m)
		var pair *BlockPair
		for n := 0; n < m.Length; n++ {
			ai, bi := convertIndices(m.AIndex+n, m.BIndex+n)
			isExactMatch := (!normalize ||
				p.aFile.GetHashOfLine(ai) == p.bFile.GetHashOfLine(bi))
			// Can we just grow the current BlockPair?
			if pair != nil {
				if pair.IsMatch == isExactMatch && pair.AIndex+pair.ALength == ai && pair.BIndex+pair.BLength == bi {
					// Yes, so just increase the length.
					glog.V(1).Info("Growing BlockPair")
					pair.ALength++
					pair.BLength++
					continue
				}
				// No, so add pair and start a new one.
				newPairs = append(newPairs, pair)
				prevPair = pair
			}
			isMove := prevPair != nil && (prevPair.AIndex+prevPair.ALength > ai || prevPair.BIndex+prevPair.BLength > bi)
			pair = &BlockPair{
				AIndex:            ai,
				ALength:           1,
				BIndex:            bi,
				BLength:           1,
				IsMatch:           isExactMatch,
				IsNormalizedMatch: !isExactMatch,
				IsMove:            isMove,
			}
		}
		newPairs = append(newPairs, pair)
		prevPair = pair
	}
	return
}

func (p *diffState) rangeToBlockPairs() (newPairs []*BlockPair) {
	// If we're here, then aRange and bRange contain the remaining lines to be
	// matched.	Figure out the subset of lines we're going to be matching.
	var aLines, bLines []LinePos
	normalize := p.config.alignNormalizedLines
	if p.config.alignRareLines {
		aLines, bLines = FindRareLinesInRanges(
			p.aRange, p.bRange, normalize,
			p.config.requireSameRarity, p.config.maxRareLineOccurrences)
		glog.V(1).Info("rangeToBlockPairs found ", len(aLines), " rare lines in A, of ",
			p.aRange.GetLineCount(), " middle lines")
		glog.V(1).Info("rangeToBlockPairs found ", len(bLines), " rare lines in B, of ",
			p.bRange.GetLineCount(), " middle lines")
		if len(aLines) == 0 || len(bLines) == 0 {
			return
		}
	} else {
		selectAll := func(lp LinePos) bool { return true }
		aLines = p.aRange.Select(selectAll)
		bLines = p.bRange.Select(selectAll)
		glog.V(1).Info("rangeToBlockPairs selected all ", len(aLines),
			" lines in A, and all ", len(bLines), " lines in B")
	}

	// These return matches in terms of the indices of aLines and bLines, not
	// the file indices, so they need to be converted afterwards.
	var matches []BlockMatch
	if p.config.detectBlockMoves {
		getHash := SelectHashGetter(normalize)
		matches = p.matchWithMoves(aLines, bLines, getHash)
	} else {
		matches = p.linearMatch(aLines, bLines, normalize)
	}

	glog.V(1).Info("rangeToBlockPairs produced ", len(matches), " BlockMatches")

	newPairs = p.convertMatchesToBlockPairs(aLines, bLines, normalize, matches)

	glog.V(1).Info("rangeToBlockPairs produced ", len(newPairs), " BlockPairs")

	return
}

// aRange and bRange contain lines to be matched; find matches between the
// two ranges (exact or normalized, possible with moves or copies).
func (p *diffState) matchRangeMiddle() {
	newPairs := p.rangeToBlockPairs()

	glog.V(1).Info("matchRangeMiddle adding ", len(newPairs), " BlockPairs")

	for _, pair := range newPairs {
		p.addBlockPair(pair)
	}
}

func (p *diffState) isExactMatch(aIndex, bIndex int) bool {
	ah, bh := p.aFile.GetHashOfLine(aIndex), p.bFile.GetHashOfLine(bIndex)
	return ah == bh
}

func (p *diffState) isExactMatchSequence(aIndex, bIndex, length int) bool {
	for length > 0 {
		if !p.isExactMatch(aIndex, bIndex) {
			return false
		}
		aIndex++
		bIndex++
		length--
	}
	return true
}

func (p *diffState) splitIfMixedMatch(n int) {
	bp := p.pairs[n]
	if !bp.IsNormalizedMatch {
		return
	}
	var runLengths []int
	var exactRuns []bool
	aIndex, bIndex, length := bp.AIndex, bp.BIndex, bp.ALength
	aLimit := aIndex + length
	for numRuns := 0; aIndex < aLimit; {
		isExact := p.isExactMatch(aIndex, bIndex)
		if numRuns == 0 || isExact != exactRuns[numRuns-1] {
			runLengths = append(runLengths, 1)
			exactRuns = append(exactRuns, isExact)
			numRuns++
		} else {
			runLengths[numRuns-1]++
		}
		aIndex++
		bIndex++
	}
	aIndex, bIndex = bp.AIndex, bp.BIndex
	for j := range runLengths {
		pair := &BlockPair{
			AIndex:            aIndex,
			ALength:           runLengths[j],
			BIndex:            bIndex,
			BLength:           runLengths[j],
			IsMatch:           exactRuns[j],
			IsMove:            false,
			IsNormalizedMatch: !exactRuns[j],
		}
		if j == 0 {
			p.pairs[n] = pair
		} else {
			p.pairs = append(p.pairs, pair)
			p.isSortedByA = false
			p.isSortedByB = false
		}
	}
}

// Some BlockPairs may be marked as normalized matches, but actually they may
// contain some or even all full matches. Split such matches; this does
// not involve adding any new lines, so don't call addBlockPair.
func (p *diffState) splitMixedMatches() {
	for n, limit := 0, len(p.pairs); n < limit; n++ {
		p.splitIfMixedMatch(n)
	}
}

func makeAOrderIndex(pairs []*BlockPair) (pairsByA []*BlockPair, pair2AOrder map[*BlockPair]int) {
	pairsByA = append(pairsByA, pairs...)
	SortBlockPairsByAIndex(pairsByA)
	pair2AOrder = make(map[*BlockPair]int)
	for n, pair := range pairsByA {
		pair2AOrder[pair] = n
	}
	return
}

func makeBOrderIndex(pairs []*BlockPair) (pairsByB []*BlockPair, pair2BOrder map[*BlockPair]int) {
	pairsByB = append(pairsByB, pairs...)
	SortBlockPairsByBIndex(pairsByB)
	pair2BOrder = make(map[*BlockPair]int)
	for n, pair := range pairsByB {
		pair2BOrder[pair] = n
	}
	return
}

func (p *diffState) getGapBetweenAIndices(p1, p2 *BlockPair) (start, length int) {
	if p1 == nil {
		start = p.aFullRange.GetStartLine()
	} else {
		start = p1.AIndex + p1.ALength
	}
	var p2Start int
	if p2 == nil {
		p2Start = p.aFile.GetLineCount()
	} else {
		p2Start = p2.AIndex
	}
	if start > p2Start {
		glog.Fatalf("p1 and p2 are out of order!\np1: %v\np2: %v", p1, p2)
	}
	return start, p2Start - start
}

func (p *diffState) getGapBetweenBIndices(p1, p2 *BlockPair) (start, length int) {
	if p1 == nil {
		start = p.bFullRange.GetStartLine()
	} else {
		start = p1.BIndex + p1.BLength
	}
	var p2Start int
	if p2 == nil {
		// TODO Consistently convert to using *FullRange instead of *File?
		p2Start = p.bFile.GetLineCount()
	} else {
		p2Start = p2.BIndex
	}
	if start > p2Start {
		glog.Fatalf("p1 and p2 are out of order!\np1: %v\np2: %v", p1, p2)
	}
	return start, p2Start - start
}

func (p *diffState) assertNoPairs() {
	if len(p.pairs) > 0 {
		glog.Fatal("Expected to find no pairs!\n", p.SDumpToDepth(3))
	}
	if p.aRange != p.aFullRange {
		glog.Fatal("Expected to aRange == aFullRange!\n", p.SDumpToDepth(3))
	}
	if p.bRange != p.bFullRange {
		glog.Fatal("Expected to bRange == bFullRange!\n", p.SDumpToDepth(3))
	}
	if p.aFile.GetFullRange() != p.aFullRange {
		glog.Fatal("Expected to aFile.GetFullRange() == aFullRange!\n", p.SDumpToDepth(3))
	}
	if p.bFile.GetFullRange() != p.bFullRange {
		glog.Fatal("Expected to bFile.GetFullRange() == bFullRange!\n", p.SDumpToDepth(3))
	}
}

func (p *diffState) computeGapInAWithB(aPair1, aPair2 *BlockPair,
	matchAPair1ToB bool) (aStart, aLength, bStart, bLength int) {
	if aPair1 == nil && aPair2 == nil {
		// There must be a complete mismatch between A and B so far.
		p.assertNoPairs()
		return 0, p.aFile.GetLineCount(), 0, p.bFile.GetLineCount()
	}
	aStart, aLength = p.getGapBetweenAIndices(aPair1, aPair2)
	if aLength == 0 {
		// There is no gap in A.
		return
	}

	var bPair1, bPair2 *BlockPair
	if matchAPair1ToB {
		if aPair1 != nil {
			bPair1 = aPair1
			i, ok := p.pair2BOrder[aPair1]
			if !ok {
				glog.Fatalf("Expected to find pair %v in pair2BOrder!", *aPair1)
			}
			if i+1 < len(p.pairsByB) {
				bPair2 = p.pairsByB[i+1]
			}
		} else {
			bPair2 = p.pairsByB[0]
		}
	} else {
		// !matchAPair1ToB, i.e. match aPair2 to B
		if aPair2 != nil {
			bPair2 = aPair2
			i, ok := p.pair2BOrder[aPair2]
			if !ok {
				glog.Fatalf("Expected to find pair %v in pair2BOrder!", *aPair2)
			}
			if i > 0 {
				bPair1 = p.pairsByB[i-1]
			}
		} else {
			bPair2 = p.pairsByB[0]
		}
	}
	bStart, bLength = p.getGapBetweenBIndices(bPair1, bPair2)
	return
}

func (p *diffState) processAllGapsInA(
	matchAPair1ToB bool, processCurrentRanges func()) {
	// TODO Handle detecting copies so that we can figure out if we need to allow
	// A lines that have been copied to be matched up with multiple B lines.
	// Or possibly vice versa, but that doesn't make sense if we assume that
	// A represents a base version and B represents a derived version, though
	// that isn't the only diff use cases.

	// Create an index of BlockPairs sorted by B.
	p.pairsByB, p.pair2BOrder = makeBOrderIndex(p.pairs)

	// Find gaps in A, and create ranges for each.
	p.sortPairsByA()
	var aRanges, bRanges []FileRange

	var prevAPair *BlockPair
	for _, thisAPair := range p.pairs {
		aStart, aLength, bStart, bLength := p.computeGapInAWithB(
			prevAPair, thisAPair, matchAPair1ToB)
		prevAPair = thisAPair

		if aLength == 0 {
			continue
		}
		aRanges = append(aRanges, CreateFileRange(p.aFile, aStart, aLength))
		bRanges = append(bRanges, CreateFileRange(p.bFile, bStart, bLength))
	}

	// Now process each of these ranges.
	for n := range aRanges {
		p.aRange = aRanges[n]
		p.bRange = bRanges[n]
		processCurrentRanges()
	}
}

func (p *diffState) computeGapInBWithA(bPair1, bPair2 *BlockPair,
	matchBPair1ToA bool) (bStart, bLength, aStart, aLength int) {
	if bPair1 == nil && bPair2 == nil {
		// There must be a complete mismatch between B and A so far.
		p.assertNoPairs()
		return 0, p.bFile.GetLineCount(), 0, p.aFile.GetLineCount()
	}
	bStart, bLength = p.getGapBetweenBIndices(bPair1, bPair2)
	if bLength == 0 {
		// There is no gap in B.
		return
	}

	var aPair1, aPair2 *BlockPair
	if matchBPair1ToA {
		if bPair1 != nil {
			aPair1 = bPair1
			i, ok := p.pair2AOrder[bPair1]
			if !ok {
				glog.Fatalf("Expected to find pair %v in pair2AOrder!", *bPair1)
			}
			if i+1 < len(p.pairsByA) {
				aPair2 = p.pairsByA[i+1]
			}
		} else {
			aPair2 = p.pairsByA[0]
		}
	} else {
		// !matchBPair1ToA, i.e. match bPair2 to A
		if bPair2 != nil {
			aPair2 = bPair2
			i, ok := p.pair2AOrder[bPair2]
			if !ok {
				glog.Fatalf("Expected to find pair %v in pair2AOrder!", *bPair2)
			}
			if i > 0 {
				aPair1 = p.pairsByA[i-1]
			}
		} else {
			aPair2 = p.pairsByA[0]
		}
	}
	aStart, aLength = p.getGapBetweenAIndices(aPair1, aPair2)
	return
}

func (p *diffState) processAllGapsInB(
	matchBPair1ToA bool, processCurrentRanges func()) {
	// Create an index of BlockPairs sorted by A.
	p.pairsByA, p.pair2AOrder = makeAOrderIndex(p.pairs)

	// Find gaps in B, and create ranges for each.
	p.sortPairsByB()
	var aRanges, bRanges []FileRange

	var prevBPair *BlockPair
	for _, thisBPair := range p.pairs {
		bStart, bLength, aStart, aLength := p.computeGapInBWithA(
			prevBPair, thisBPair, matchBPair1ToA)
		prevBPair = thisBPair
		if bLength == 0 {
			continue
		}
		aRanges = append(aRanges, CreateFileRange(p.aFile, aStart, aLength))
		bRanges = append(bRanges, CreateFileRange(p.bFile, bStart, bLength))
	}

	// Now process each of these ranges.
	for n := range aRanges {
		p.aRange = aRanges[n]
		p.bRange = bRanges[n]
		processCurrentRanges()
	}

	return
}

// Fill any gaps when sorted by A.
func (p *diffState) fillAGaps() {
	p.sortPairsByA()
	ai := 0
	for n, limit := 0, len(p.pairs); n < limit; n++ {
		bp := p.pairs[n]
		if ai < bp.AIndex {
			// Found a gap.
			var bi int
			if n == 0 {
				// Gap at the start.
				bi = 0
			} else if p.pairs[n-1].BIndex < bp.BIndex {
				bi = p.pairs[n-1].BIndex + p.pairs[n-1].BLength
			} else {
				// A move exists at this point.
				bi = bp.BIndex
			}
			pair := &BlockPair{
				AIndex:            ai,
				ALength:           bp.AIndex - ai,
				BIndex:            bi,
				BLength:           0,
				IsMatch:           false,
				IsMove:            false,
				IsNormalizedMatch: false,
			}
			if glog.V(1) {
				glog.Infof("fillAGaps inserting: %v", *pair)
			}
			p.addBlockPair(pair)
		}
		ai = bp.AIndex + bp.ALength
	}
	if ai < p.aFile.GetLineCount() {
		// Gap at the end.
		pair := &BlockPair{
			AIndex:            ai,
			ALength:           p.aFile.GetLineCount() - ai,
			BIndex:            p.bFile.GetLineCount(),
			BLength:           0,
			IsMatch:           false,
			IsMove:            false,
			IsNormalizedMatch: false,
		}
		if glog.V(1) {
			glog.Infof("fillAGaps inserting at the end: %v", *pair)
		}
		p.addBlockPair(pair)
	}
}

// Fill any gaps when sorted by B.
func (p *diffState) fillBGaps() {
	p.sortPairsByB()
	bi := 0
	for n, limit := 0, len(p.pairs); n < limit; n++ {
		bp := p.pairs[n]
		if bi < bp.BIndex {
			// Found a gap.
			var ai int
			if n == 0 {
				// Gap at the start.
				ai = 0
			} else if p.pairs[n-1].AIndex < bp.AIndex {
				ai = p.pairs[n-1].AIndex + p.pairs[n-1].ALength
			} else {
				// A move exists at this point.
				ai = bp.AIndex
			}
			pair := &BlockPair{
				AIndex:            ai,
				ALength:           0,
				BIndex:            bi,
				BLength:           bp.BIndex - bi,
				IsMatch:           false,
				IsMove:            false,
				IsNormalizedMatch: false,
			}
			if glog.V(1) {
				glog.Infof("fillBGaps inserting: %v", *pair)
			}
			p.addBlockPair(pair)
		}
		bi = bp.BIndex + bp.BLength
	}
	if bi < p.bFile.GetLineCount() {
		// Gap at the end.
		pair := &BlockPair{
			AIndex:            p.aFile.GetLineCount(),
			ALength:           0,
			BIndex:            bi,
			BLength:           p.bFile.GetLineCount() - bi,
			IsMatch:           false,
			IsMove:            false,
			IsNormalizedMatch: false,
		}
		if glog.V(1) {
			glog.Infof("fillBGaps inserting at the end: %v", *pair)
		}
		p.addBlockPair(pair)
	}
}

func (p *diffState) fillAllGaps() {
	p.fillAGaps()
	p.fillBGaps()
}
