package dm

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

func PerformDiff(aFile, bFile *File, config DifferencerConfig) (pairs []*BlockPair) {
	p := diffState{
		aFile:           aFile,
		bFile:           bFile,
		aRange:          aFile.GetFullRange(),
		bRange:          bFile.GetFullRange(),
		aRemainingCount: aFile.GetLineCount(),
		bRemainingCount: bFile.GetLineCount(),
		config:          config,
	}

	glog.Info("PerformDiff entry, diffState:\n", p.SDumpToDepth(1))

	if config.matchEnds {
		if !(p.exactMatchCommonPrefix() && p.exactMatchCommonSuffix()) {
			// All matching done.
			return p.getPairsToReturn()
		}
		if config.matchNormalizedEnds {
			n := len(p.pairs)
			keepGoing := (p.normalizedMatchCommonPrefix() && p.normalizedMatchCommonPrefix())
			if n < len(p.pairs) {
      	// Split normalized matches that contain full matches.
		    p.splitMixedMatches()
			}
			if !keepGoing {
				// All matching done.
				return p.getPairsToReturn()
			}
		}
	}

	// Figure out an alignment of the remains after prefix and suffix matching.
  n := len(p.pairs)
	p.matchMiddle()

  if n < len(p.pairs) && config.alignRareLines {
    // Extend any matches that can be extended.
    p.extendAllMatches()
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
//   we might want to replace one entry with several);
// * forward iterate over the members, tolerating modifications (e.g. filling
//   gaps while iterating, or splitting an entry);
// * get the last member, and possibly the first;
// * lookup members (e.g. when filling gaps by AIndex, we'll need to find
//   the neighbors of the BlockPair in B).

type diffState struct {
	// Full files
	aFile, bFile *File

	// Range being considered. Reduced by common prefix and suffix matching.
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

func FileRangeIsEmpty(r FileRange) bool {
	return r == nil || r.GetLineCount() == 0
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
    p.isSortedByB = false  // Might be, but can't be sure.
  }
}

func (p *diffState) sortPairsByB() {
  if !p.isSortedByB {
    SortBlockPairsByBIndex(p.pairs)
    p.isSortedByA = false   // Might be, but can't be sure.
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
		glog.Infof("addBlockPair: aRemainingCount=%d   bRemainingCount=%d",
		p.aRemainingCount, p.bRemainingCount)
	}
	return p.isMatchingComplete()
}

func (p *diffState) addBlockMatch(m BlockMatch, normalizedMatch bool) bool {
	// Convert to a BlockPair.  Not yet checking to see if the normalized match
	// is also a full match, or may have some full match and some normalized match
	// lines; will convert all of them later.
	pair := &BlockPair{
		AIndex:            m.AIndex,
		ALength:           m.Length,
		BIndex:            m.BIndex,
		BLength:           m.Length,
		IsMatch:           !normalizedMatch,
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

// These operations all return false if they know that
// the differencing is done, else they return true.

type endMatcher func(aRange, bRange FileRange, normalized bool) (
	aRest, bRest FileRange, bp *BlockPair)

func (p *diffState) commonEndMatcher(fn endMatcher, normalized bool) bool {
	// Assuming here that p.aRange and p.bRange are non-empty.
	aRange, bRange := p.aRange, p.bRange

	glog.Infof("commonEndMatcher(%v, %v)  lines: %d  and  %d", fn, normalized,
			aRange.GetLineCount(), bRange.GetLineCount())

	var bp *BlockPair
	p.aRange, p.bRange, bp = fn(aRange, bRange, normalized)

	if p.aRange == nil {
		glog.Infof("commonEndMatcher: matched ALL %d lines of aRange", aRange.GetLineCount())
	} else {
		before, after := aRange.GetLineCount(), p.aRange.GetLineCount()
		if after != before {
			glog.Infof("commonEndMatcher: matched %d lines of %d, leaving %d  (aRange)",
				before - after, before, after)
		}
	}

	if p.bRange == nil {
		glog.Infof("commonEndMatcher: matched ALL %d lines of bRange", bRange.GetLineCount())
	} else {
		before, after := bRange.GetLineCount(), p.bRange.GetLineCount()
		if after != before {
			glog.Infof("commonEndMatcher: matched %d lines of %d, leaving %d  (bRange)",
				before - after, before, after)
		}
	}

	p.addBlockPair(bp)

	return !(FileRangeIsEmpty(p.aRange) || FileRangeIsEmpty(p.bRange))
}

func (p *diffState) exactMatchCommonPrefix() bool {
	return p.commonEndMatcher(MatchCommonPrefix, false)
}

func (p *diffState) exactMatchCommonSuffix() bool {
	return p.commonEndMatcher(MatchCommonSuffix, false)
}

func (p *diffState) normalizedMatchCommonPrefix() bool {
	return p.commonEndMatcher(MatchCommonPrefix, true)
}

func (p *diffState) normalizedMatchCommonSuffix() bool {
	return p.commonEndMatcher(MatchCommonSuffix, true)
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
			AIndex: ab.Index1,  // Indices in aLines and bLines, respectively.
			BIndex: ab.Index2,
			Length: 1,
		})
	}
	return result
}

func (p *diffState) matchMiddle() {
	// If we're here, then aRange and bRange contain the remaining lines to be
	// matched.  Figure out the subset of lines we're going to be matching.
	var aLines, bLines []LinePos
	normalize := p.config.alignNormalizedLines
	if p.config.alignRareLines {
		aLines, bLines = FindRareLinesInRanges(
			p.aRange, p.bRange, normalize,
			p.config.requireSameRarity, p.config.maxRareLineOccurrences)
		glog.V(1).Info("matchMiddle found ", len(aLines), " rare lines in A, of ",
				p.aRange.GetLineCount(), " middle lines")
		glog.V(1).Info("matchMiddle found ", len(bLines), " rare lines in B, of ",
				p.bRange.GetLineCount(), " middle lines")
	} else {
		selectAll := func(lp LinePos) bool { return true }
		aLines = p.aRange.Select(selectAll)
		bLines = p.bRange.Select(selectAll)
		glog.V(1).Info("matchMiddle selected all ", len(aLines), " lines in A, and all ",
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

	glog.V(1).Info("matchMiddle produced ", len(matches), " BlockMatches")

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
			  AIndex:            ai,
			  ALength:           1,
			  BIndex:            bi,
			  BLength:           1,
			  IsMatch:           isExactMatch,
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
    return 0, p2.AIndex
  }
  start = p1.AIndex + p1.ALength
  if p2 == nil {
    return start, p.aFile.GetLineCount() - start
  }
  if start <= p2.AIndex {
    return beyond, p2.AIndex - beyond
  } else {
    return -888888888, -999999999 // Not a valid situation. Panic instead?
  }
}

func (p *diffState) getGapBetweenBIndices(p1, p2 *BlockPair) (start, length int) {
  if p1 == nil {
    return 0, p2.BIndex
  }
  start = p1.BIndex + p1.BLength
  if p2 == nil {
    return start, p.bFile.GetLineCount() - start
  }
  if start <= p2.BIndex {
    return beyond, p2.BIndex - beyond
  } else {
    return -888888888, -999999999 // Not a valid situation. Panic instead?
  }
}

/*
func (p *diffState) commonGapMatcher(as, al, bs, bl int, fn endMatcher, normalized bool) {
  if a

  aRange := CreateFileRange(p.aFile, as, al)
  
  
  
   *File, start, length int) FileRange {



	// Assuming here that p.aRange and p.bRange are non-empty.
	aRange, bRange := p.aRange, p.bRange





func (p *diffState) commonEndMatcher(fn endMatcher, normalized bool) bool {
	// Assuming here that p.aRange and p.bRange are non-empty.
	aRange, bRange := p.aRange, p.bRange

	glog.Infof("commonEndMatcher(%v, %v)  lines: %d  and  %d", fn, normalized,
			aRange.GetLineCount(), bRange.GetLineCount())

	var bp *BlockPair
	p.aRange, p.bRange, bp = fn(aRange, bRange, normalized)

	if p.aRange == nil {
		glog.Infof("commonEndMatcher: matched ALL %d lines of aRange", aRange.GetLineCount())
	} else {
		before, after := aRange.GetLineCount(), p.aRange.GetLineCount()
		if after != before {
			glog.Infof("commonEndMatcher: matched %d lines of %d, leaving %d  (aRange)",
				before - after, before, after)
		}
	}

	if p.bRange == nil {
		glog.Infof("commonEndMatcher: matched ALL %d lines of bRange", bRange.GetLineCount())
	} else {
		before, after := bRange.GetLineCount(), p.bRange.GetLineCount()
		if after != before {
			glog.Infof("commonEndMatcher: matched %d lines of %d, leaving %d  (bRange)",
				before - after, before, after)
		}
	}

	p.addBlockPair(bp)

	return !(FileRangeIsEmpty(p.aRange) || FileRangeIsEmpty(p.bRange))
}

func (p *diffState) exactMatchCommonPrefix() bool {
	return p.commonEndMatcher(MatchCommonPrefix, false)
}

func (p *diffState) exactMatchCommonSuffix() bool {
	return p.commonEndMatcher(MatchCommonSuffix, false)
}

func (p *diffState) normalizedMatchCommonPrefix() bool {
	return p.commonEndMatcher(MatchCommonPrefix, true)
}

func (p *diffState) normalizedMatchCommonSuffix() bool {
	return p.commonEndMatcher(MatchCommonSuffix, true)
}
*/


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
			AIndex:            ai,
			ALength:           bp.AIndex - ai,
			BIndex:            bi,
			BLength:           0,
			IsMatch:           false,
			IsMove:            false,
			IsNormalizedMatch: false,
		}


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
				AIndex:            ai,
				ALength:           bp.AIndex - ai,
				BIndex:            bi,
				BLength:           0,
				IsMatch:           false,
				IsMove:            false,
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

