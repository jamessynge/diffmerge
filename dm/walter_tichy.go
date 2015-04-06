package dm

import ()

// Walter Tichy devised diff and merge techniques while developing RCS. This
// file has some crude implementations of his "Basic" algorithms, from
// "The String-to-String Correction Problem with Block Moves", Perdue, 1983.

// TODO Improve in the case where the lines are known to be unique (combining
// Bram Cohen's ideas regarding Patience Diff with Tichy's). In particular,
// I'm wondering whether I can weight the matches not just by their length,
// but by how rare the lines in the match are (e.g. 1/frequency as a weight,
// though that might be rather aggressive).

func prefixMatchLength(aLines, bLines []LinePos, aOffset, bOffset int) (
	matchLength int) {
	for aOffset < len(aLines) && bOffset < len(bLines) {
		if aLines[aOffset].Hash != bLines[bOffset].Hash {
			break
		}
		matchLength++
		aOffset++
		bOffset++
	}
	return
}

// longestPrefixMatch finds the location and length of the longest match
// between bLines[bOffset:] and aLines. Does not assume that a and b consist
// of unique, or even remotely unique, lines, which can be used to optimize
// the search considerably.
func longestPrefixMatch(aLines, bLines []LinePos, bOffset int) (
	aLongestPrefixOffset, maxLength int) {
	aOffset := 0
	// Continue while it possible that there is a longer match.
	for aOffset+maxLength <= len(aLines) && bOffset+maxLength < len(bLines) {
		// Determine length of prefix match between aLines[aOffset:] and
		// bLines[bOffset:].
		pml := prefixMatchLength(aLines, bLines, aOffset, bOffset)
		if pml > maxLength {
			// New maximum found.
			aLongestPrefixOffset = aOffset
			maxLength = pml
		}
		aOffset++
	}
	return
}

// Find maximal blocks that can be matched between a and b, where each line
// in b is matched with at most one in a; vice versa is not necessarily true,
// unless a and b consist only of lines that are locally unique (i.e. no hash
// appears twice in a, nor twice in b).
func BasicTichyMaximalBlockMoves(aLines, bLines []LinePos) []BlockMove {
	var result []BlockMove
	bOffset := 0
	for bOffset < len(bLines) {
		aLongestPrefixOffset, prefixLength := longestPrefixMatch(
				aLines, bLines, bOffset)
		if prefixLength > 0 {
			result = append(result, BlockMove{
				AOffset: aLongestPrefixOffset,
				BOffset: bOffset,
				Length:  prefixLength,
			})
			bOffset += prefixLength
		} else {
			bOffset++
		}
	}
	return result
}













	// Find the longest common substring (consecutive lines, block of lines)
	// between aLines and bLines (which already have their common prefixes and
	// suffixes removed; I'm sure that is optimal from a performance perspective,
	// but I'm not sure that it will always produce the longest common substring
	// of the original aLines and bLines).


