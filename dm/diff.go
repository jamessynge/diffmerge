package dm

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

// TODO Work on support for block COPIES.

// TODO How can we use sentinal lines here? Maybe produce a new FileRange
// that has unique sentinals at the start and end of each Full FileRange;
// or common start sentinals at the start of every Full FileRange, and
// common end sentinals at the end of every Full FileRange, so that we'd be
// guaranteed of a match at the start, and then not worry about gap filling
// before BOF or after EOF.

func PerformDiff(aFile, bFile *File, config DifferencerConfig) (pairs []*BlockPair) {
	p := diffState{
		aFile:					 aFile,
		bFile:					 bFile,
		aFullRange:					aFile.GetFullRange(),
		bFullRange:					bFile.GetFullRange(),
		aRange:					aFile.GetFullRange(),
		bRange:					bFile.GetFullRange(),
		aRemainingCount: aFile.GetLineCount(),
		bRemainingCount: bFile.GetLineCount(),
		config:					config,
	}

	glog.Info("PerformDiff entry, diffState:\n", p.SDumpToDepth(1))

	if config.matchEnds {
		if p.matchRangeEnds(/*prefix*/ true, /*suffix*/ true, /*normalize*/ false) {
			// All matching done. The files must be equal.
			return p.getPairsToReturn()
		}
		if (config.matchNormalizedEnds &&
		 		p.matchRangeEnds(/*prefix*/ true, /*suffix*/ true, /*normalize*/ true)) {
			// All matching done. The files must be equal after normalization.
			return p.getPairsToReturn()
		}
	}

	// Figure out an alignment of the remains after prefix and suffix matching.
	n := len(p.pairs)
	p.matchRangeMiddle()

	if n < len(p.pairs) && config.alignRareLines {
		// Extend any matches that can be extended.
		p.extendAllMatches()	// TODO Maybe change this to just be fillAllGaps,
													// which will find gaps, then do matchRangeEnds
													// on all gaps, then go back over the remaining gaps
													// and run matchRangeMiddle on each of those gaps.
	}

	if !p.isMatchingComplete() {
		p.fillAllGaps()
	}

	return p.getPairsToReturn()
}

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

type diffState struct {
	// Full files
	aFile, bFile *File
	aFullRange, bFullRange FileRange

	// Range being considered by matchRangeEnds (to match common prefix and suffix
	// lines), matchRangeMiddle (to find matches with possible mismatches
	// between them). Set by extendAllMatches and fillAllGaps. 
	aRange, bRange FileRange

	// Discovered pairs (matches, moves, mismatches); we don't support (i.e.
	// detect) copies, so we can do counting of lines in pairs to track our
	// progress.
	pairs []*BlockPair

	// Sort order, if known, of pairs
	isSortedByA, isSortedByB bool

	// Set when filling gaps.
	pairsByB []*BlockPair
	pair2BOrder map[*BlockPair]int

	// Number of lines not yet represented in pairs.
	aRemainingCount, bRemainingCount int

	detectedAMove bool

	// Controls operation.
	config DifferencerConfig
}


func (p *diffState) SDumpToDepth(depth int) string {
	var cs spew.ConfigState = spew.Config
	cs.MaxDepth = depth
	return cs.Sdump(p)
}

func (p *diffState) isMatchingComplete() bool {
	// This approach assumes that no lines will be copied.
	// TODO Ideally we'd be able to detect copies, and even better would be
	// to detect changes within the copies.
	return p.aRemainingCount == 0 || p.bRemainingCount == 0
}

func (p *diffState) sortPairsByA() {
	if !p.isSortedByA {
		SortBlockPairsByAIndex(p.pairs)
		p.isSortedByA = true
		p.isSortedByB = false	// Might be, but can't be sure.
	}
}

func (p *diffState) sortPairsByB() {
	if !p.isSortedByB {
		SortBlockPairsByBIndex(p.pairs)
		p.isSortedByA = false	 // Might be, but can't be sure.
		p.isSortedByB = true
	}
}

// Returns false (stop) if one or both of the remaining counts drops to zero,
// else returns true (keep going).
func (p *diffState) addBlockPair(bp *BlockPair) bool {
	// TODO Add optional checking for validity: correct line indices, and not
	// overlapping existing BlockPairs. For now, just make sure we don't have
	// fewer than zero lines remaining.
	if bp != nil {
		glog.Infof("addBlockPair: bp:\n%s", spew.Sdump(bp))
		if len(p.pairs) > 0 {
			lastPair := p.pairs[len(p.pairs) - 1]
			if p.isSortedByA && bp.AIndex < lastPair.AIndex + lastPair.ALength {
				p.isSortedByA = false
			}
			if p.isSortedByB && bp.BIndex < lastPair.BIndex + lastPair.BLength {
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
		p.aRemainingCount -= bp.ALength
		p.bRemainingCount -= bp.BLength
		if p.aRemainingCount < 0 || p.bRemainingCount < 0 {
			glog.Fatalf("Adding BlockPair dropped a remaining count below zero!\n"+
				"BlockPair: %v\ndiffState: %v", *bp, *p)
		}
		glog.Infof("addBlockPair: aRemainingCount=%d	 bRemainingCount=%d",
		p.aRemainingCount, p.bRemainingCount)
	}
	return p.isMatchingComplete()
}

func (p *diffState) addBlockMatch(m BlockMatch, normalizedMatch bool) bool {
	// Convert to a BlockPair.	Not yet checking to see if the normalized match
	// is also a full match, or may have some full match and some normalized match
	// lines; will convert all of them later.
	pair := &BlockPair{
		AIndex:						m.AIndex,
		ALength:					 m.Length,
		BIndex:						m.BIndex,
		BLength:					 m.Length,
		IsMatch:					 !normalizedMatch,
		IsNormalizedMatch: normalizedMatch,
	}
	if glog.V(1) {
		glog.Infof("addBlockMatch inserting: %v", *pair)
	}
	return p.addBlockPair(pair)
}

func (p *diffState) getPairsToReturn() []*BlockPair {
	if p.aRemainingCount > 0 {
		if p.bRemainingCount > 0 {
			glog.Fatalf(
				"getPairsToReturn Not ready to return yet!\ndiffState:\n%s",
				p.SDumpToDepth(2))
		}
		p.fillAGaps();
	} else if p.bRemainingCount > 0 {
		p.fillBGaps();
	}
	SortBlockPairsByAIndex(p.pairs)
	// TODO Split normalized matches that contain full matches.
	return p.pairs
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
	return p.aRange.IsEmpty() || p.bRange.IsEmpty()
}

func convertSliceIndicesToFileIndices(
	aLines, bLines []LinePos, matches []BlockMatch) {
	for n := range matches {
		// Indices in aLines and bLines, respectively.
		ai, bi := matches[n].AIndex, matches[n].BIndex

		// Now line numbers in A and B.
		ai, bi = aLines[ai].Index, bLines[bi].Index

		matches[n].AIndex, matches[n].BIndex = ai, bi
	}
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
			AIndex: ab.Index1,	// Indices in aLines and bLines, respectively.
			BIndex: ab.Index2,
			Length: 1,
		})
	}
	return result
}

func (p *diffState) matchRangeMiddle() {
	// If we're here, then aRange and bRange contain the remaining lines to be
	// matched.	Figure out the subset of lines we're going to be matching.
	var aLines, bLines []LinePos
	normalize := p.config.alignNormalizedLines
	if p.config.alignRareLines {
		aLines, bLines = FindRareLinesInRanges(
			p.aRange, p.bRange, normalize,
			p.config.requireSameRarity, p.config.maxRareLineOccurrences)
		glog.V(1).Info("matchRangeMiddle found ", len(aLines), " rare lines in A, of ",
				p.aRange.GetLineCount(), " middle lines")
		glog.V(1).Info("matchRangeMiddle found ", len(bLines), " rare lines in B, of ",
				p.bRange.GetLineCount(), " middle lines")
	} else {
		selectAll := func(lp LinePos) bool { return true }
		aLines = p.aRange.Select(selectAll)
		bLines = p.bRange.Select(selectAll)
		glog.V(1).Info("matchRangeMiddle selected all ", len(aLines), " lines in A, and all ",
			len(bLines), " lines in B")
	}

	// TODO Move rest of function into matchWithMoves and linearMatch,
	// add helpers they can share as necessary.

	// These return matches in terms of the indices of aLines and bLines, not
	// the file indices, so they need to be converted afterwards.
	var matches []BlockMatch
	if p.config.detectBlockMoves {
		getHash := SelectHashGetter(normalize)
		matches = p.matchWithMoves(aLines, bLines, getHash)
	} else {
		matches = p.linearMatch(aLines, bLines, normalize)
	}

	glog.V(1).Info("matchRangeMiddle produced ", len(matches), " BlockMatches")

	// Convert these to BlockPairs. Since they aren't necessarily contiguous,
	// and there might be moves, we'll process them line by line, building
	// BlockPairs that are contiguous.

	convertIndices := func(ai, bi int) (aIndex, bIndex int) {
		return aLines[ai].Index, bLines[bi].Index
	}

	SortBlockMatchesByAIndex(matches)
	var prevPair *BlockPair
	for i, m := range matches {
		glog.V(1).Infof("matches[%d] = %v", i, m)
		var pair *BlockPair
		for n := 0; n < m.Length; n++ {
			ai, bi := convertIndices(m.AIndex + n, m.BIndex + n)
			isExactMatch := (!normalize ||
					p.aFile.GetHashOfLine(ai) == p.bFile.GetHashOfLine(bi))
			// Can we just grow the current BlockPair?
			if pair != nil {
				if pair.IsMatch == isExactMatch && pair.AIndex + pair.ALength == ai && pair.BIndex + pair.BLength == bi {
					// Yes, so just increase the length.
					glog.V(1).Info("Growing BlockPair")
					pair.ALength++
					pair.BLength++
					continue
				}
				// No, so add pair and start a new one.
				p.addBlockPair(pair)
				prevPair = pair
			}
			isMove := prevPair != nil && (prevPair.AIndex + prevPair.ALength > ai || prevPair.BIndex + prevPair.BLength > bi)
			pair = &BlockPair{
				AIndex:						ai,
				ALength:					 1,
				BIndex:						bi,
				BLength:					 1,
				IsMatch:					 isExactMatch,
				IsNormalizedMatch: !isExactMatch,
				IsMove: isMove,
			}
		}
		p.addBlockPair(pair)
		prevPair = pair
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
			AIndex:						aIndex,
			ALength:					 runLengths[j],
			BIndex:						bIndex,
			BLength:					 runLengths[j],
			IsMatch:					 exactRuns[j],
			IsMove:						false,
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

func makeBOrderIndex(pairs []*BlockPair) (pairsByB []*BlockPair, pair2BOrder map[*BlockPair]int) {
	pairsByB = append(pairsByBy, pairs...)
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
		p2Start = p.AIndex
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
		p2Start = p.BIndex
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

func (p *diffState) fillGapBetweenPairPair(aPair1, aPair2, bPair1, bPair2 *BlockPair) {
	aStart, aLength := p.getGapBetweenAIndices(aPair1, aPair2)

	i, ok := p.pair2BOrder[p1]
	if !ok { panic("Where is the missing pair?") }
	p3 := pairsByB[i]
	var bs, bl int
	bs = p3.BIndex + p3.BLength
	if i >= limit {
		bl = p.bFile.GetLineCount() - bs
	} else {
		p4 := pairsByB[i + 1]
		bl = p4.BIndex - bs
	}

	if al <= 0 {
		if bl <= 0 {
			// There isn't a gap.
			continue
		}
		// There is a gap in A only.
		pair := &BlockPair{
			AIndex:						ai,
			ALength:					 bp.AIndex - ai,
			BIndex:						bi,
			BLength:					 0,
			IsMatch:					 false,
			IsMove:						false,
			IsNormalizedMatch: false,
		}
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
			if i + 1 < len(p.pairsByB) {
				bPair2 = p.pairsByB[i + 1]
			}
		} else {
			bPair2 = p.pair2BOrder[0]
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
				bPair1 = p.pairsByB[i - 1]
			}
		} else {
			bPair2 = p.pair2BOrder[0]
		}
	}
	bStart, bLength = getGapBetweenBIndices(bPair1, bPair2)
	return
}

func (p *diffState) processGapInA(aPair1, aPair2 *BlockPair,
			matchAPair1ToB bool, processCurrentRanges func()) {
	aStart, aLength, bStart, bLength := p.computeGapInAWithB(
			aPair1, aPair2, matchAPair1ToB)

	if aLength == 0 { return }

	p.aRange = CreateFileRange(p.aFile, aStart, aLength)
	p.bRange = CreateFileRange(p.bFile, bStart, bLength)

	processCurrentRanges()
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
				aPair1, aPair2, matchAPair1ToB)
		prevAPair = thisAPair	

		if aLength == 0 { continue }
		aRanges = append(aRanges, CreateFileRange(p.aFile, aStart, aLength))
		bRanges = append(bRanges, CreateFileRange(p.bFile, bStart, bLength))
	}

	// Now process each of these ranges.

	var newPairs []*BlockPair








	for n, limit := 0, len(p.pairs) - 1; n < limit; n++ {
		p1, p2 := p.pairs[n], p.pairs[n + 1]
		as, al := getGapBetweenAIndices(p1, p2)
		if al


}




// Attempt to shrink all gaps by extending existing exact or normalized matches,
// or creating new normalized or exact matches (respectively) adjacent to the
// existing matches.
func (p *diffState) extendAllMatches() {
	// Create an index of BlockPairs sorted by B.
	p.pairsByB, p.pair2BOrder := makeBOrderIndex(p.pairs)

	// Find a gap, then try to shrink it; then repeat.
	// Start with extending them forward.
	p.sortPairsByA()
	for n, limit := 0, len(p.pairs) - 1; n < limit; n++ {
		p1, p2 := p.pairs[n], p.pairs[n + 1]
		as, al := getGapBetweenAIndices(p1, p2)

		i, ok := pair2BOrder[p1]
		if !ok { panic("Where is the missing pair?") }
		p3 := pairsByB[i]
		var bs, bl int
		bs = p3.BIndex + p3.BLength
		if i >= limit {
			bl = p.bFile.GetLineCount() - bs
		} else {
			p4 := pairsByB[i + 1]
			bl = p4.BIndex - bs
		}

		if al <= 0 {
			if bl <= 0 {
				// There isn't a gap.
				continue
			}
			// There is a gap in A only.
			pair := &BlockPair{
				AIndex:						ai,
				ALength:					 bp.AIndex - ai,
				BIndex:						bi,
				BLength:					 0,
				IsMatch:					 false,
				IsMove:						false,
				IsNormalizedMatch: false,
			}

			// There isn't a gap.
			continue
		}
p.config.alignRareLines


		if bl <= 0 { continue }




		// 
			
		if 
		j, ok := pair2BOrder[p1]
		

		if n
	
	
	}
	for n, pair := range p.pairs {
		if n
	
	
	}

	
	



	
	
	
/*	
	 in matchesByB, allowing
	// us to detect adjacency in B when processing matchesByA.
	matches2BOrder := make(map[BlockMatch]int)
	for n := range p.matchesByB {
		matches2BOrder[p.matchesByB[n]] = n
	}

	isAdjacentToPredecessor := func(aOrder, bOrder int) bool {
		if aOrder == 0 {
			return bOrder == 0
		}
		return matches2BOrder[p.matchesByA[aOrder-1]] == bOrder-1
	}





*/











	p.pairsByB, p.pair2BOrder = nil, nil
	
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
				AIndex:						ai,
				ALength:					 bp.AIndex - ai,
				BIndex:						bi,
				BLength:					 0,
				IsMatch:					 false,
				IsMove:						false,
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
			AIndex:						ai,
			ALength:					 p.aFile.GetLineCount() - ai,
			BIndex:						p.bFile.GetLineCount(),
			BLength:					 0,
			IsMatch:					 false,
			IsMove:						false,
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
				AIndex:						ai,
				ALength:					 0,
				BIndex:						bi,
				BLength:					 bp.BIndex - bi,
				IsMatch:					 false,
				IsMove:						false,
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
			AIndex:						p.aFile.GetLineCount(),
			ALength:					 0,
			BIndex:						bi,
			BLength:					 p.bFile.GetLineCount() - bi,
			IsMatch:					 false,
			IsMove:						false,
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

