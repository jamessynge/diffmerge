package dm

import (
	"log"
)

// Patience Diff, devised by Bram Cohen, focuses on the lines that are unique
// within the files, rather than diff(1) which can be confused by the
// potentially large number of identical lines (e.g. blank lines, lines containing
// "return" or "}"). This helps to identify coarse alignments, and then
// we can recurse in the gaps.  See these sources for more info:
//
// https://bramcohen.livejournal.com/73318.html - Patience Diff Advantages
// https://alfedenzo.livejournal.com/170301.html - Patience Diff, a brief summary
// http://bryanpendleton.blogspot.com/2010/05/patience-diff.html
// https://en.wikipedia.org/wiki/Patience_sorting
//
// What worries me is that it doesn't seem to handle the "block move" situation,
// which Walter Tichy's approach does handle; while block moves aren't as
// common as most other changes (e.g. adding and changing a few lines),
// they're still fairly common, and quite challenging to deal with if you're
// doing a big refactoring while others are busy making additions to the old
// code.
//
// Cohen's overview:
// 1) Match the first lines of both if they're identical, then match the second,
//    third, etc. until a pair doesn't match.
// 2) Match the last lines of both if they're identical, then match the next to
//    last, second to last, etc. until a pair doesn't match.
// 3) Find all lines which occur exactly once on both sides, then do longest
//    common subsequence on those lines, matching them up.
// 4) Do steps 1-2 on each section between matched lines.

func matchCommonEnds(aLines, bLines []LinePos) (
	commonEnds []BlockMatch, aMiddle, bMiddle []LinePos) {
	// Find all lines at the start that are the same (the common prefix).
	length, limit := 0, minInt(len(aLines), len(bLines))
	if limit <= 0 {
		return
	}
	for ; length < limit; length++ {
		if aLines[length].Hash != bLines[length].Hash {
			break
		}
	}
	if length > 0 {
		// There is a common prefix.
		commonEnds = append(commonEnds, BlockMatch{
			AIndex: aLines[0].Index,
			BIndex: bLines[0].Index,
			Length: length,
		})
		if length == limit {
			return
		}
		aLines = aLines[length:]
		bLines = bLines[length:]
		limit -= length
	}

	// Find all lines at the end that are the same (the common suffix).
	length = 0
	aOffset := len(aLines) - 1
	bOffset := len(bLines) - 1
	for length < limit {
		if aLines[aOffset].Hash != bLines[bOffset].Hash {
			break
		}
		length++
		aOffset--
		bOffset--
	}
	if length > 0 {
		// There is a common suffix.
		aOffset++
		bOffset++
		commonEnds = append(commonEnds, BlockMatch{
			AIndex: aLines[aOffset].Index,
			BIndex: bLines[bOffset].Index,
			Length: length,
		})
		aLines = aLines[:aOffset]
		bLines = bLines[:bOffset]
	}
	return commonEnds, aLines, bLines
}

type indicesAndHash struct {
	aIndex, bIndex int
	hash           uint32
}

func longestCommonSubsequenceOfRareLines(aLines, bLines []LinePos,
	aCounts, bCounts map[uint32]int, maxCount int) (
	aLCSLines, bLCSLines []LinePos) {
	// Determine which lines are equally rare (with fewer than maxCount
	// occurrences) in the two sequences aLines and bLines.
	// Decided to require the counts to be the same as this simplifies
	// the reasoning about the possible matches.
	isRare := func(count int) bool {
		return 1 <= count && count <= maxCount
	}
	rareLineKeys := make(map[uint32]bool)
	for h, aCount := range aCounts {
		if isRare(aCount) && aCount == bCounts[h] {
			rareLineKeys[h] = true
		}
	}
	if len(rareLineKeys) == 0 {
		return
	}
	selector := func(lp LinePos) bool {
		return rareLineKeys[lp.Hash]
	}
	aRareLines := selectLines(aLines, selector)
	bRareLines := selectLines(bLines, selector)

	// Build an index from hash to aLines entries.
	aRareLineMap := make(map[uint32][]int)
	for n := range aRareLines {
		h := aRareLines[n].Hash
		aRareLineMap[h] = append(aRareLineMap[h], n)
	}

	// Build an array that records the hashes of the bLines, and their
	// corresponding positions in the aLines.
	var ihs []indicesAndHash
	for bIndex := range bRareLines {
		h := bRareLines[bIndex].Hash
		aIndices := aRareLineMap[h]
		if len(aIndices) <= 0 {
			panic("expected a line in a to match this line in b")
		}
		aIndex := aIndices[0]
		aRareLineMap[h] = aIndices[1:]
		ihs = append(ihs, indicesAndHash{aIndex, bIndex, h})
	}

	// We now know the correspondence in indices between a and b, and thus
	// have the equally rare lines of a in the same order as they appear in b,
	// along with their indices in both sequences of rare lines.
	// Apply the patience sorting algorithm to find the longest common
	// subsequence; note that we don't actually try to determine the full sort,
	// just the first longest common subsequence.
	// TODO Can probably just have piles be of type []int, and contain just the
	// aIndex values.  Similarly, ihses can probably just be an array of aIndex
	// values, sorted in the order in which aRareLines entries appear in bRareLines.

	var piles [][]indicesAndHash
	backPointers := make([][]int, 1) // Length of previous pile when next pile
	// is added to
	for _, ih := range ihs {
		// Add ihs to the first pile that has a smaller aIndex.
		addTo := 0
		for ; addTo < len(piles); addTo++ {
			topIndex := len(piles[addTo]) - 1
			if ih.aIndex < piles[addTo][topIndex].aIndex {
				// We've found a pile we can place it on.
				break
			}
		}
		// Are we making a new pile?
		if addTo == len(piles) {
			// Yes, add an empty array.
			piles = append(piles, nil)
			backPointers = append(backPointers, nil)
		}
		// Add ihs to the selected pile.
		piles[addTo] = append(piles[addTo], ih)
		// And record the height of the previous pile.
		if addTo > 0 {
			backPointers[addTo] = append(backPointers[addTo], len(piles[addTo-1]))
		}
	}

	// The longest common subsequence is of length len(piles).
	lcs := make([]indicesAndHash, len(piles))
	pileIndex := len(piles) - 1
	indexInPile := len(piles[pileIndex]) - 1
	for {
		lcs[pileIndex] = piles[pileIndex][indexInPile]
		if pileIndex == 0 {
			break
		}
		indexInPile = backPointers[pileIndex][indexInPile] - 1
		pileIndex--
	}

	// Finally we can construct aLCSLines and bLCSLines
	for _, ih := range lcs {
		aLCSLines = append(aLCSLines, aRareLines[ih.aIndex])
		bLCSLines = append(bLCSLines, bRareLines[ih.bIndex])
	}
	return
}

// Compute a longest common subsequence that has enough lines to be
// trustworthy. We've already trimmed the common prefix and suffix.
func getLongestCommonSubsequenceOfRareLines(aLines, bLines []LinePos) (
	aLCSLines, bLCSLines []LinePos) {
	aCounts := countLineOccurrences(aLines)
	bCounts := countLineOccurrences(bLines)

	// Compute the LCS of rare lines in aLines and bLines, where rare starts
	// with unique, but then grows to include more common lines if necessary
	// until the length of the LCS is at least targetLength (rather arbitrarily
	// chosen).
	targetLength := minInt(minInt(len(aLines), len(bLines))/16, 10)
	lcsLength := 0
	for maxCount := 1; maxCount <= 5; maxCount++ {
		aLCSLines, bLCSLines = longestCommonSubsequenceOfRareLines(
			aLines, bLines, aCounts, bCounts, maxCount)
		lcsLength = len(aLCSLines)
		// Arbitrary ending critera.
		if lcsLength >= targetLength {
			return
		}
	}
	// Note that there may be no common lines.
	return
}

// If the lines immediately before those in the block move are identical,
// then grow the block move by one and repeat.
func (p *BlockMatch) GrowBackwards(aLines, bLines []LinePos) {
	aLimit, bLimit := aLines[0].Index, bLines[0].Index
	a := findLineWithIndex(aLines, p.AIndex)
	b := findLineWithIndex(bLines, p.BIndex)

	if aLines[a].Hash != bLines[b].Hash {
		log.Fatalf("Lines %d and %d should have the same hash", a, b)
	}

	growBy := 0
	for a > aLimit && b > bLimit {
		a--
		b--
		if aLines[a].Hash != bLines[b].Hash {
			break
		}
		growBy++
	}
	p.AIndex -= growBy
	p.BIndex -= growBy
	p.Length += growBy
}

// If the lines immediately after those in the block move are identical,
// then grow the block move by one and repeat.
func (p *BlockMatch) GrowForwards(aLines, bLines []LinePos) {
	aLimit, bLimit := aLines[len(aLines)-1].Index, bLines[len(bLines)-1].Index
	a := findLineWithIndex(aLines, p.AIndex+p.Length-1)
	b := findLineWithIndex(bLines, p.BIndex+p.Length-1)

	if aLines[a].Hash != bLines[b].Hash {
		log.Fatalf("Lines %d and %d should have the same hash", a, b)
	}

	growBy := 0
	for a < aLimit && b < bLimit {
		a++
		b++
		if aLines[a].Hash != bLines[b].Hash {
			break
		}
		growBy++
	}
	p.Length += growBy
}

func BramCohensPatienceDiff(aFile, bFile *File) []BlockMatch {
	commonEnds, aMiddle, bMiddle := matchCommonEnds(aFile.Lines, bFile.Lines)

	aLCSLines, bLCSLines := getLongestCommonSubsequenceOfRareLines(
		aMiddle, bMiddle)

	var middleBlockMoves []BlockMatch
	for lcsIndex := 0; lcsIndex < len(aLCSLines); {
		bm := BlockMatch{
			AIndex: aLCSLines[lcsIndex].Index,
			BIndex: bLCSLines[lcsIndex].Index,
			Length: 1,
		}
		bm.GrowBackwards(aMiddle, bMiddle)
		bm.GrowForwards(aMiddle, bMiddle)
		// Has the block grown to consume any of the following LCS entries? If so,
		// skip past them.
		beyondA := bm.AIndex + bm.Length
		beyondB := bm.BIndex + bm.Length
		lcsIndex++
		for lcsIndex < len(aLCSLines) {
			isBeyondA := aLCSLines[lcsIndex].Index >= beyondA
			isBeyondB := bLCSLines[lcsIndex].Index >= beyondB
			if isBeyondA != isBeyondB {
				log.Fatalf("Unexpected:\nLCS a: %v\nLCS b: %v\nBlockMove: %v",
					aLCSLines[lcsIndex], bLCSLines[lcsIndex], bm)
			}
			if !isBeyondA {
				// Reached the end of a block of equal pairs of lines before we
				// got to this next LCS entry.
				break
			}
			// Consume this entry.
			lcsIndex++
			continue
		}
		middleBlockMoves = append(middleBlockMoves, bm)
	}

	if len(commonEnds) == 2 {
		var result []BlockMatch
		result = append(result, commonEnds[0])
		result = append(result, middleBlockMoves...)
		return append(result, commonEnds[1])
	}
	if len(commonEnds) == 0 {
		return middleBlockMoves
	}
	if commonEnds[0].AIndex == 0 && commonEnds[0].BIndex == 0 {
		return append(commonEnds, middleBlockMoves...)
	} else {
		return append(middleBlockMoves, commonEnds[0])
	}
}

/*




// Find matching blocks between aLines and bLines starting at both ends,
// then finding the longest common substring in the middle; then recursing
// for the gaps. Not so great at finding moves.
func recursivePatienceDiff(aLines, bLines []LinePos) (result []BlockMove) {
	// Find all lines at the start that are the same (the common prefix).
	cp, limit := 0, minInt(len(aLines), len(bLines))
	if limit <= 0 { return }
	for ; cp < limit ; cp++ {
		if aLines[cp].Hash != bLines[cp].Hash {
			break
		}
	}
	if cp > 0 {
		// There is a common prefix.
		result = append(result, BlockMove{
			AOffset: aLines[0].Index,
			BOffset: bLines[0].Index,
			Length: cp,
		})
		if cp == limit { return }
		aLines = aLines[cp:]
		bLines = bLines[cp:]
	}

	// Find all lines at the end that are the same (the common suffix).
	cs := 0
	aOffset := len(aLines) - 1
	bOffset := len(bLines) - 1
	for cs < limit {
		if aLines[aOffset].Hash != bLines[bOffset].Hash {
			break
		}
		cs++
		aOffset--
		bOffset--
	}
	if cs > 0 {
		// There is a common suffix.
		aOffset++
		bOffset++
		// Do the common prefix and suffix overlap?
		if cp > 0 && ((cp + cs) > len(aLines) || (cp + cs) > len(bLines)) {
			// Yes, they do, but since we checked above for cp having been the full
			// length of either of a or b, we know that there must be a gap for
			// one of them. Keep the longest, and then recurse.
			if cp >= cs {
				result = append(result,
						recursivePatienceDiff(aLines[cp:], bLines[cp:])...)
				return
			} else {
				otherResult := recursivePatienceDiff(aLines[:aOffset], bLines[:bOffset])
				return append(otherResult, result...)
			}
		}
		// They don't overlap.
		result = append(result, BlockMove{
			AOffset: aLines[aOffset].Index,
			BOffset: bLines[bOffset].Index,
			Length: cs,
		})
		aLines = aLines[cp:aOffset]
		bLines = bLines[cp:bOffset]
	} else if cp > 0 {
		aLines = aLines[cp:]
		bLines = bLines[cp:]
	}

	// For Patience Diff, we next find the longest common sub-SEQUENCE, which
	// means an ordering of lines that both a and b share in common, but not
	// necessarily consecutive lines (i.e. the strings "artsy fartsy" and
	// "art party" have a common subsequence of "art arty").


	panic("nyi")


	return
}












*/
