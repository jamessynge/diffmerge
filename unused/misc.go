package unused

import (
	"github.com/golang/glog"

	"github.com/jamessynge/diffmerge/dm"
)

func maxInt(i, j int) int {
	if i < j {
		return j
	} else {
		return i
	}
}

func convertSliceIndicesToFileIndices(
	aLines, bLines []dm.LinePos, matches []dm.BlockMatch) {
	for n := range matches {
		// Indices in aLines and bLines, respectively.
		ai, bi := matches[n].AIndex, matches[n].BIndex

		// Now line numbers in A and B.
		ai, bi = aLines[ai].Index, bLines[bi].Index

		matches[n].AIndex, matches[n].BIndex = ai, bi
	}
}

func selectLines(lines []dm.LinePos, fn func(lp dm.LinePos) bool) []dm.LinePos {
	var result []dm.LinePos
	for n := range lines {
		if fn(lines[n]) {
			result = append(result, lines[n])
		}
	}
	return result
}

func selectCommonUniqueLines(aLines, bLines []dm.LinePos, counts map[uint32]int) (
	aUniqueLines, bUniqueLines []dm.LinePos) {
	fn := func(lp dm.LinePos) bool {
		return counts[lp.Hash] == 1
	}
	return selectLines(aLines, fn), selectLines(bLines, fn)
}

// Which line hashes are unique within their source lines
// AND common between the two sets of lines.
func intersectionOfUniqueLines(aCounts, bCounts map[uint32]int) (
	commonUniqueCounts map[uint32]int) {
	commonUniqueCounts = make(map[uint32]int)
	for h, aCount := range aCounts {
		if aCount == 1 && bCounts[h] == 1 {
			commonUniqueCounts[h] = 1
		}
	}
	return
}

// Returns the count of occurrences of the lines.
func countLineOccurrences(lines []dm.LinePos, getHash func(lp dm.LinePos) uint32) (counts map[uint32]int) {
	counts = make(map[uint32]int)
	for n := range lines {
		counts[getHash(lines[n])]++
	}
	return
}

func findLineWithIndex(lines []dm.LinePos, index int) int {
	for lo, hi := 0, len(lines)-1; lo <= hi; {
		mid := (lo + hi) / 2
		if index < lines[mid].Index {
			hi = mid - 1
		} else if index > lines[mid].Index {
			lo = mid + 1
		} else {
			return mid
		}
	}
	glog.Fatalf("Failed to find line with index %d in:\n%v", index, lines)
	return -1
}

func SwapBlockMatches(a []dm.BlockMatch) {
	for n := range a {
		a[n].AIndex, a[n].BIndex = a[n].BIndex, a[n].AIndex
	}
}

func SwapBlockPairs(a []*dm.BlockPair) {
	for n := range a {
		a[n].AIndex, a[n].BIndex = a[n].BIndex, a[n].AIndex
		a[n].ALength, a[n].BLength = a[n].BLength, a[n].ALength
	}
}
