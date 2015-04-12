package dm

import (
	"github.com/golang/glog"
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

	glog.Infof("matchCommonEnds: %d lines from A, %d lines from B", len(aLines), len(bLines))

	// Find all lines at the start that are the same (the common prefix).
	length, limit := 0, minInt(len(aLines), len(bLines))
	if limit <= 0 {
		return
	}

	glog.Infof("matchCommonEnds: lines [%d, %d) of A", aLines[0].Index, aLines[len(aLines)-1].Index)
	glog.Infof("matchCommonEnds: lines [%d, %d) of B", bLines[0].Index, bLines[len(bLines)-1].Index)

	for ; length < limit; length++ {
		if aLines[length].Hash != bLines[length].Hash {

			glog.Infof("matchCommonEnds: common prefix ends at offset %d", length)
			glog.Infof("matchCommonEnds: A of mismatch: %v", aLines[length])
			glog.Infof("matchCommonEnds: B of mismatch: %v", bLines[length])

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

			glog.Infof("matchCommonEnds: common suffix ends at offset %d", length)
			glog.Infof("matchCommonEnds: A of mismatch: %v", aLines[aOffset])
			glog.Infof("matchCommonEnds: B of mismatch: %v", bLines[bOffset])

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
	aRareIndex, bRareIndex int
	hash                   uint32
}

func longestCommonSubsequenceOfRareLines(aLines, bLines []LinePos,
	aCounts, bCounts map[uint32]int, exactMatch bool, maxCount int) (
	aLCSLines, bLCSLines []LinePos) {
	// Determine which lines are equally rare (with fewer than maxCount
	// occurrences) in the two sequences aLines and bLines.
	// Decided to require the counts to be the same as this simplifies
	// the reasoning about the possible matches.

	normalized := ""
	getHash := func(lp LinePos) uint32 { return lp.Hash }
	if !exactMatch {
		normalized = ", normalized"
		getHash = func(lp LinePos) uint32 { return lp.NormalizedHash }
	}

	glog.Infof("longestCommonSubsequenceOfRareLines: %d lines from A, %d lines from B, %d is max count for rare lines%s", len(aLines), len(bLines), maxCount, normalized)

	rareLineKeys := make(map[uint32]bool)
	for h, aCount := range aCounts {
		if 1 <= aCount && aCount <= maxCount && aCount == bCounts[h] {
			rareLineKeys[h] = true
		}
	}
	if len(rareLineKeys) == 0 {
		glog.Infof("len(rareLineKeys) == 0")
		return
	}

	selector := func(lp LinePos) bool {
		return rareLineKeys[getHash(lp)]
	}
	aRareLines := selectLines(aLines, selector)
	bRareLines := selectLines(bLines, selector)

	glog.Infof("longestCommonSubsequenceOfRareLines: %d rare lines from A, %d rare lines from B", len(aRareLines), len(bRareLines))

	// Build an index from hash to aRareLines entries.
	aRareLineMap := make(map[uint32][]int)
	for aRareIndex := range aRareLines {
		h := getHash(aRareLines[aRareIndex])
		aRareLineMap[h] = append(aRareLineMap[h], aRareIndex)
	}

	glog.Infof("aRareLineMap: %v", aRareLineMap)

	// Build an array that records the hashes of the bLines, and their
	// corresponding positions in the aLines.
	var ihs []indicesAndHash
	for bRareIndex := range bRareLines {
		h := getHash(bRareLines[bRareIndex])
		aRareLineIndices := aRareLineMap[h]
		if len(aRareLineIndices) <= 0 {
			glog.Fatal("expected a line in a to match this line in b")
		}
		// Pop the first entry off of aRareLineIndices
		aRareIndex := aRareLineIndices[0]
		aRareLineMap[h] = aRareLineIndices[1:]
		ihs = append(ihs, indicesAndHash{aRareIndex, bRareIndex, h})
	}

	glog.Infof("ihs: %v", ihs)
	glog.Infof("aRareLineMap: %v", aRareLineMap)

	// We now know the correspondence in indices between a and b, and thus
	// have the equally rare lines of a in the same order as they appear in b,
	// along with their indices in both sequences of rare lines.
	// Apply the patience sorting algorithm to find the longest common
	// subsequence; note that we don't actually try to determine the full sort,
	// just the first longest common subsequence.
	// TODO Can probably just have piles be of type []int, and contain just the
	// aRareIndex values.  Similarly, ihses can probably just be an array of aRareIndex
	// values, sorted in the order in which aRareLines entries appear in bRareLines.

	var piles [][]indicesAndHash
	backPointers := make([][]int, 1) // Length of previous pile when next pile
	// is added to
	for _, ih := range ihs {
		// Add ihs to the first pile that has a smaller aRareIndex.
		addTo := 0
		for ; addTo < len(piles); addTo++ {
			topIndex := len(piles[addTo]) - 1
			if ih.aRareIndex < piles[addTo][topIndex].aRareIndex {
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

	glog.Infof("piles: %v", piles)
	glog.Infof("backPointers: %v", backPointers)

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
	glog.Infof("lcs: %v", lcs)

	// Finally we can construct aLCSLines and bLCSLines
	for _, ih := range lcs {
		aLCSLines = append(aLCSLines, aRareLines[ih.aRareIndex])
		bLCSLines = append(bLCSLines, bRareLines[ih.bRareIndex])
	}
	glog.Infof("aLCSLines: %v", aLCSLines)
	glog.Infof("bLCSLines: %v", bLCSLines)
	return
}

// Compute a longest common subsequence that has enough lines to be
// trustworthy. We've already trimmed the common prefix and suffix.
func getLongestCommonSubsequenceOfRareLines(aLines, bLines []LinePos) (
	aLCSLines, bLCSLines []LinePos) {
	minLinesSize := minInt(len(aLines), len(bLines))
	glog.Infof("minLinesSize=%d", minLinesSize)
	if minLinesSize == 0 {
		return
	}

	aHashCounts := countLineOccurrences(aLines, GetLPHash)
	bHashCounts := countLineOccurrences(bLines, GetLPHash)
	minHashesSize := minInt(len(aHashCounts), len(bHashCounts))
	glog.Infof("minHashesSize=%d", minHashesSize)

	// TODO Delay computing this until we need it.
	aNormalizedCounts := countLineOccurrences(aLines, GetLPNormalizedHash)
	bNormalizedCounts := countLineOccurrences(bLines, GetLPNormalizedHash)
	minNormalizedSize := minInt(len(aNormalizedCounts), len(bNormalizedCounts))
	glog.Infof("minNormalizedSize=%d", minNormalizedSize)

	// TODO Pass targetLength to longestCommonSubsequenceOfRareLines
	// so it stops if it is impossible to achieve.
	// Should I start with the larger maxCounts, and go down? Or just use
	// N (e.g. 3) and never search?

	// Compute the LCS of rare lines in aLines and bLines, where rare starts
	// with unique, but then grows to include more common lines if necessary
	// until the length of the LCS is at least targetLength (rather arbitrarily
	// chosen).
	targetLength := minInt(minLinesSize/2, maxInt(minLinesSize/16, 5))
	glog.Infof("minLinesSize=%d,   targetLength=%d", minLinesSize, targetLength)
	lcsLength := 0
	for maxCount := 1; maxCount <= 5; maxCount++ {
		aLCSLines, bLCSLines = longestCommonSubsequenceOfRareLines(
			aLines, bLines, aHashCounts, bHashCounts, true, maxCount)
		lcsLength = len(aLCSLines)
		// Arbitrary ending critera.
		if lcsLength >= targetLength {
			glog.Infof("Found enough LCS entries: %d > %d", lcsLength, targetLength)
			return
		}
		// Try again, but matching normalized lines.
		aLCSLines, bLCSLines = longestCommonSubsequenceOfRareLines(
			aLines, bLines, aNormalizedCounts, bNormalizedCounts, false, maxCount)
		lcsLength = len(aLCSLines)
		// Arbitrary ending critera.
		if lcsLength >= targetLength {
			glog.Infof("Found enough normalized LCS entries: %d > %d", lcsLength, targetLength)
			return
		}
	}
	// Note that there may be no common lines.
	return
}

// If the lines immediately before those in the block move are identical,
// then grow the block move by one and repeat.
func (p *BlockMatch) GrowBackwards(aLines, bLines []LinePos) {
	glog.Infof("GrowBackwards: BlockMatch = %v", *p)

	aLimit, bLimit := aLines[0].Index, bLines[0].Index
	glog.Infof("GrowBackwards aLimit=%d, bLimit=%d", aLimit, bLimit)

	a := findLineWithIndex(aLines, p.AIndex)
	b := findLineWithIndex(bLines, p.BIndex)
	glog.Infof("GrowBackwards a=%d, b=%d", a, b)

	if aLines[a].Hash != bLines[b].Hash {
		glog.Fatalf("GrowBackwards: Lines %d and %d should have the same hash", a, b)
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

	glog.Infof("GrowBackwards growBy=%d", growBy)

	p.AIndex -= growBy
	p.BIndex -= growBy
	p.Length += growBy
}

// If the lines immediately after those in the block move are identical,
// then grow the block move by one and repeat.
func (p *BlockMatch) GrowForwards(aLines, bLines []LinePos) {
	glog.Infof("GrowForwards: BlockMatch = %v", *p)

	aLimit := aLines[len(aLines)-1].Index
	bLimit := bLines[len(bLines)-1].Index
	glog.Infof("GrowForwards aLimit=%d, bLimit=%d", aLimit, bLimit)

	a := findLineWithIndex(aLines, p.AIndex+p.Length-1)
	b := findLineWithIndex(bLines, p.BIndex+p.Length-1)
	glog.Infof("GrowForwards a=%d, b=%d", a, b)

	if aLines[a].Hash != bLines[b].Hash {
		glog.Fatalf("GrowForwards: Lines %d and %d should have the same hash", a, b)
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

	glog.Infof("GrowForwards growBy=%d", growBy)

	p.Length += growBy
}

func BramCohensPatienceDiff(aFile, bFile *File) []BlockMatch {
	commonEnds, aMiddle, bMiddle := matchCommonEnds(aFile.Lines, bFile.Lines)

	aLCSLines, bLCSLines := getLongestCommonSubsequenceOfRareLines(
		aMiddle, bMiddle)

	glog.Infof("aLCSLines: %v", aLCSLines)
	glog.Infof("bLCSLines: %v", bLCSLines)

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
		glog.Infof("Grew BlockMatch to %v", bm)
		beyondA := bm.AIndex + bm.Length
		beyondB := bm.BIndex + bm.Length

		lcsIndex++
		for lcsIndex < len(aLCSLines) {
			glog.Infof("Following LCS entries: %v   AND   %v", aLCSLines[lcsIndex], bLCSLines[lcsIndex])

			isBeyondA := aLCSLines[lcsIndex].Index >= beyondA
			isBeyondB := bLCSLines[lcsIndex].Index >= beyondB

			glog.Infof("isBeyondA = %v,   isBeyondB = %v", isBeyondA, isBeyondB)

			if isBeyondA != isBeyondB {
				glog.Fatalf("Unexpected:\nLCS a: %v\nLCS b: %v\nBlockMove: %v",
					aLCSLines[lcsIndex], bLCSLines[lcsIndex], bm)
			}
			if isBeyondA {
				// Reached the end of a block of equal pairs of lines before we
				// got to this next LCS entry.
				break
			}
			// Consume this entry.
			glog.Infof("Consuming following LCS entries")
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
