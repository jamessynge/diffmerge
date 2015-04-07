package dm

import (
	"log"
	"sort"
)

func minInt(i, j int) int {
	if i < j {
		return i
	} else {
		return j
	}
}

func findLineWithIndex(lines []LinePos, index int) int {
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

// BlockMatchByAIndex implements sort.Interface for []BlockMatch based on
// the AIndex field, then BIndex.
type BlockMatchByAIndex []BlockMatch

func (a BlockMatchByAIndex) Len() int           { return len(a) }
func (a BlockMatchByAIndex) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BlockMatchByAIndex) Less(i, j int) bool {
  if a[i].AIndex != a[j].AIndex {
    return a[i].AIndex < a[j].AIndex
  }
  return a[i].BIndex < a[j].BIndex
}

func SortBlockMatchesByAIndex(a []BlockMatch) {
  sort.Sort(BlockMatchByAIndex(a))
}

func SwapBlockMatches(a []BlockMatch) {
  for n := range a {
    a[n].AIndex, a[n].BIndex = a[n].BIndex, a[n].AIndex
  }
}



