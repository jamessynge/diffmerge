package dm

import (
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

	if config.matchEnds {
		if !(p.exactMatchCommonPrefix() && p.exactMatchCommonSuffix()) {
			// All matching done.
			return p.getPairsToReturn()
		}
		if config.matchNormalizedEnds {
			n := len(p.pairs)
			keepGoing := (p.normalizedMatchCommonPrefix() && p.normalizedMatchCommonPrefix())
			if n < len(p.pairs) {
				// TODO Split normalized matches that contain full matches.
				glog.Info("TODO Split normalized matches that contain full matches.")
			}
			if !keepGoing {
				// All matching done.
				return p.getPairsToReturn()
			}
		}
	}

	// Figure out an alignment of the remains after prefix and suffix matching.
	p.matchMiddle()

	// Split any matches that shouldn't be together.
	if config.matchNormalizedEnds || config.alignNormalizedLines {
		p.splitMixedMatches()
	}

	if !p.isMatchingComplete() {
		// Extend any exact matches that can be extended, forward at first, then
		// backward.
		// TODO Should the backward extension wait until after the
		// forward normalized extension occurs?
		p.fillGaps()
	}

	return p.getPairsToReturn()
}

type diffState struct {
	// Full files
	aFile, bFile *File

	// Range being considered. Reduced by common prefix and suffix matching.
	aRange, bRange FileRange

	// Discovered pairs (matches, moves, mismatches); we don't support (i.e.
	// detect) copies, so we can do counting of lines in pairs to track our
	// progress.
	pairs []*BlockPair

	// Number of lines not yet represented in pairs.
	aRemainingCount, bRemainingCount int

	detectedAMove bool

	// Controls operation.
	config DifferencerConfig
}

func FileRangeIsEmpty(r FileRange) bool {
	return r == nil || r.GetLineCount() == 0
}

func (p *diffState) isMatchingComplete() bool {
	return p.aRemainingCount > 0 && p.bRemainingCount > 0
}

// Returns false (stop) if one or both of the remaining counts drops to zero,
// else returns true (keep going).
func (p *diffState) addBlockPair(bp *BlockPair) bool {
	// TODO Add optional checking for validity: correct line indices, and not
	// overlapping existing BlockPairs. For now, just make sure we don't have
	// fewer than zero lines remaining.
	if bp != nil {
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
		IsMatch: !normalizedMatch,
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
			glog.Fatalf("getPairsToReturn Not ready to return yet!\ndiffState: %v", *p)
		}
		// Sort by A, fill any gaps.
		SortBlockPairsByAIndex(p.pairs)
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
					glog.Infof("getPairsToReturn inserting: %v", *pair)
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
				glog.Infof("getPairsToReturn inserting at the end: %v", *pair)
			}
			p.addBlockPair(pair)
		}
	} else if p.bRemainingCount > 0 {
		// Sort by B, fill any gaps.
		SortBlockPairsByBIndex(p.pairs)
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
					glog.Infof("getPairsToReturn inserting: %v", *pair)
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
				glog.Infof("getPairsToReturn inserting at the end: %v", *pair)
			}
			p.addBlockPair(pair)
		}
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
	var bp *BlockPair
	p.aRange, p.bRange, bp = fn(aRange, bRange, normalized)
	if bp != nil {
		p.pairs = append(p.pairs, bp)
	}
	return FileRangeIsEmpty(p.aRange) || FileRangeIsEmpty(p.bRange)
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

func (p *diffState) matchWithMoves(
		aLines, bLines []LinePos,
		getHash func(lp LinePos) uint32) []BlockMatch {
	matches := BasicTichyMaximalBlockMoves(aLines, bLines, getHash)
	return matches
}

func (p *diffState) linearMatch(
		aLines, bLines []LinePos,
		getHash func(lp LinePos) uint32) []BlockMatch {

panic("NYI")
return nil
}


func (p *diffState) matchMiddle() {
	// If we're here, then aRange and bRange contains the remaining lines to be
	// matched.  Figure out the subset of lines we're going to be matching.

	var aLines, bLines []LinePos
	if p.config.alignRareLines {
		aLines, bLines = FindRareLinesInRanges(
				p.aRange, p.bRange, p.config.alignNormalizedLines,
				p.config.requireSameRarity, p.config.maxRareLineOccurrences)
	} else {
		selectAll := func(lp LinePos) bool { return true }
		aLines = p.aRange.Select(selectAll)
		bLines = p.bRange.Select(selectAll)
	}

	getHash := SelectHashGetter(p.config.alignNormalizedLines)

	var matches []BlockMatch
	if p.config.detectBlockMoves {
		matches = p.matchWithMoves(aLines, bLines, getHash)
	} else {
		matches = p.linearMatch(aLines, bLines, getHash)
	}

	// Convert these to BlockPairs
	for _, m := range matches {
		p.addBlockMatch(m, p.config.alignNormalizedLines)
	}
}

func (p *diffState) isExactMatch(aIndex, bIndex int) bool {
	ah, bh := p.aFile.GetHashOfLine(aIndex), p.bFile.GetHashOfLine(bIndex)
	return ah == bh
}

func (p *diffState) isExactMatchSequence(aIndex, bIndex, length int) bool {
	for length > 0 {
		if !p.isExactMatch(aIndex, bIndex) { return false }
		aIndex++
		bIndex++
		length--
	}
	return true
}

func (p *diffState) splitIfMixedMatch(n int) {
	bp := p.pairs[n]
	if !bp.IsNormalizedMatch { return }
	var runLengths []int
	var exactRuns []bool
	aIndex, bIndex, length := bp.AIndex, bp.BIndex, bp.ALength
	aLimit := aIndex + length
	for numRuns := 0; aIndex < aLimit; {
		isExact := p.isExactMatch(aIndex, bIndex)
		if numRuns == 0 || isExact != exactRuns[numRuns - 1] {
			runLengths = append(runLengths, 1)
			exactRuns = append(exactRuns, isExact)
			numRuns++
		} else {
			runLengths[numRuns - 1]++
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


func (p *diffState) fillGaps() {
}

