package dm

import (
"log"
)


func minInt(i, j int) int {
	if i < j {
		return i
	} else {
		return j
	}
}

func findLineWithIndex(lines []LinePos, index int) int {
	for lo, hi := 0, len(lines) - 1; lo <= hi; {
		mid := (lo + hi) / 2
		if index < lines[mid].Index {
			hi = mid - 1
		} else if index > lines[mid].Index {
			lo = mid + 1
		} else {
			return mid
		}
	}
	log.Fatalf("Failed to find line with index %s in:\n%v", index, lines)
	return -1
}

func selectLines(lines []LinePos, fn func(lp LinePos) bool) []LinePos {
	var result []LinePos
	for n := range lines {
		if fn(lines[n]) {
			result = append(result, lines[n])
		}
	}
	return result
}

// Returns the count of occurrences of the lines.
func countLineOccurrences(lines []LinePos) (counts map[uint32]int) {
	counts = make(map[uint32]int)
	for n := range lines {
		counts[lines[n].Hash]++
	}
	return
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

func selectCommonUniqueLines(aLines, bLines []LinePos, counts map[uint32]int) (
		aUniqueLines, bUniqueLines []LinePos) {
	fn := func(lp LinePos) bool {
		return counts[lp.Hash] == 1
	}
	return selectLines(aLines, fn), selectLines(bLines, fn)
}

