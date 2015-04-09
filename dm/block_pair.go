package dm

import (
	"fmt"
	"io"

	"github.com/golang/glog"
)
/*
// If the line immediately before those in the block move are the same
// after normalization, then add those to the block and repeat.
func (p *BlockPair) GrowBackwards(aLines, bLines []LinePos) {
	glog.Infof("GrowBackwards: BlockPair = %v", *p)

	aLimit, bLimit := aLines[0].Index, bLines[0].Index
	glog.Infof("GrowBackwards aLimit=%d, bLimit=%d", aLimit, bLimit)

	a := findLineWithIndex(aLines, p.AIndex)
	b := findLineWithIndex(bLines, p.BIndex)
	glog.Infof("GrowBackwards a=%d, b=%d", a, b)

	if p.ALength > 0 && p.BLength > 0 
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
func (p *BlockPair) GrowForwards(aLines, bLines []LinePos) {
	glog.Infof("GrowForwards: BlockPair = %v", *p)

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
*/


// Define BlockMatches U and V in matches to be "adjacent matches"
// when there exist integers i and j such that:
//      matchesByA[i] == U      matchesByA[i + 1] == V
//      matchesByB[j] == U      matchesByB[j + 1] == V
// We want to identify such adjacent matches because we can then be
// confident in emiting a conflict, insertion or deletion between
// two, rather than there being a likelihood that the gap between the
// two represents a move.


type matchesToPairsState struct {
aFile, bFile *File
aLineCount, bLineCount int

matchesByA, matchesByB []BlockMatch
hasMoves bool

		unmatchedAs, unmatchedBs map[int]bool

	pairs []BlockPair









}

// We assume that A is the primary file.  If not, then caller should take
// care of swapping before and after calling.

func InitMatchesToPairsState(aFile, bFile *File, matches []BlockMatch) *matchesToPairsState {
	p := &matchesToPairsState{
		aFile: aFile,
		bFile: bFile,
		aLineCount: len(aFile.Lines),
		bLineCount: len(bFile.Lines),
		matchesByA: append([]BlockMatch(nil), matches...),
		unmatchedAs: make(map[int]bool),
		unmatchedBs: make(map[int]bool),
	}

	// To simplify processing at the end, add sentinal: an empty match that is
	// at the end of the two files, and another one before the two files.
	// TODO Determine if the sentinals help.
	p.matchesByA = append(p.matchesByA, BlockMatch{
		AIndex: -1,
		BIndex: -1,
		Length: 1,
	})
	p.matchesByA = append(p.matchesByA, BlockMatch{
		AIndex: aLineCount,
		BIndex: bLineCount,
		Length: 0,
	})

	SortBlockMatchesByAIndex(p.matchesByA)
	p.matchesByB = append([]BlockMatch(nil), p.matchesByA...)
	SortBlockMatchesByBIndex(p.matchesByB)

	return p
}

func (p *matchesToPairsState) fillGap(loA, hiA, loB, hiB) {
	// Is there a common prefix (identical or normalized)?
	identical := true
	limit := intMin(hiA - loA, hiB - loB)
	prefix := 0
	for ; prefix < limit; prefix++ {
		lpA := &p.aFile.Lines[loA + prefix]
		lpB := &p.bFile.Lines[loB + prefix]
		if lpA.Hash != lpB.Hash {
			if lpA.NormalizedHash != lpB.NormalizedHash {
				break
			}
			// TODO If prefix > 0 && identical, emit a common prefix that represents
			// an identical match, then continue with the normalized match.
			identical = false
		}
	}
	if prefix > 0 {
		p.pairs = append(p.pairs, BlockPair{
			AIndex:  loA,
			ALength: prefix,
			BIndex:  loB,
			BLength: prefix,
			IsMatch: identical,
			IsMove:  false,
			IsNormalizedMatch: !identical,
		})
		if prefix >= limit {
			return
		}
		loA += prefix
		loB += prefix
		limit -= prefix
	}

	// Is there a common suffix (identical or normalized)?
	suffix := 1
	identical = true
	for ; suffix <= limit; suffix++ {
		lpA := &p.aFile.Lines[hiA - suffix]
		lpB := &p.bFile.Lines[hiB - suffix]
		if lpA.Hash != lpB.Hash {
			if lpA.NormalizedHash != lpB.NormalizedHash {
				break
			}
			// TODO If suffix > 1 && identical, emit a common suffix that represents
			// an identical match, then continue with the normalized match.
			identical = false
		}
	}
	if suffix > 1 {
		suffix--
		p.pairs = append(p.pairs, BlockPair{
			AIndex:  hiA - suffix,
			ALength: suffix,
			BIndex:  hiB - suffix,
			BLength: suffix,
			IsMatch: identical,
			IsMove:  false,
			IsNormalizedMatch: !identical,
		})
		hiA -= suffix
		hiB -= suffix
	}
	
	p.pairs = append(p.pairs, BlockPair{
		AIndex:  loA,
		ALength: hiA,
		BIndex:  loB,
		BLength: hiB,
		IsMatch: false,
		IsMove:  false,
		IsNormalizedMatch: false,
	})
}

func (p *matchesToPairsState) createPairsByA() {
	// Create an index from BlockMatch to position in matchesByB, allowing
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

	// March through the BlockMatches
	// creating BlockPairs representing matches, moves, and mismatches
	// (insertions, deletions and conflicts).
	// We don't emit a non-match when a move is encountered (i.e. between
	// two sequential matches in p.matchesByA that are not sequential in
	// p.matchesByB).

	loA, loB := 0, 0
	p.hasMoves = false
	for aOrder := range p.matchesByA {
		ma := p.matchesByA[aOrder]
		bOrder := matches2BOrder[ma]
		isMove := true
		if isAdjacentToPredecessor(aOrder, bOrder) {
			isMove = false
			// No gaps to fill for sentinal at start
			if ma.AIndex >= 0 {
				p.fillGap(loA, ma.AIndex, loB, ma.BIndex)
			}
		} else {
			p.hasMoves = true
			for n := loA; n < ma.AIndex; n++ {
				p.unmatchedAs[n] = true
			}
		}
		// Emit a match.
		pairs = append(pairs, BlockPair{
			AIndex:  ma.AIndex,
			ALength: ma.Length,
			BIndex:  ma.BIndex,
			BLength: ma.Length,
			IsMatch: true,
			IsMove:  isMove,
		})
	}
}

func (p *matchesToPairsState) fillGapsByB() {
	if p.hasMoves {
		// There shouldn't be any gaps in B if there have been no moves.
		return
	}

	SortBlockPairsByBIndex(p.pairs)

	// Find any gaps in B, attempt to fill them.
	loB = 0
	for n, numPairs := 0, len(p.pairs); n < numPairs; n++ {
		pair := &p.pairs[n]
		if loB < pair.BIndex {
			// There is a gap. Find the lines in A just before this pair
			// that are also unmatched.
			hiA := pair.AIndex
			loA := hiA - 1
			for ; loA >= 0 && p.unmatchedAs[loA]; loAa-- {
				delete(p.unmatchedAs, loA)
			}
			loA++
			// TODO Consider skipping the common suffix matching here.
			p.fillGap(loA, hiA, loB, pair.BIndex)
		}
		loB = pair.BIndex + pair.BLength
	}
}

func (p *matchesToPairsState) fillGapsByA() {
	if len(p.unmatchedAs) == 0 {
		return
	}










// Confused routine to create the blocks that we'll output, representing
// matches between files, inserted lines, dropped lines, and conflicts.
// Because I want to support block moves (and by extension block copies),
// I may have lines from one file appearing twice. To simplify things I'm
// treating one file as primary, where its lines will appear in the output
// exactly once and in order, and the other will be secondary, and its lines
// will appear out of order IFF there have been moves.
func BlockMatchesToBlockPairs(
	matches []BlockMatch, aIsPrimary bool, aLineCount, bLineCount int) (
	pairs []BlockPair) {
	glog.Infof("BlockMatchesToBlockPairs")
	matchesByA := append([]BlockMatch(nil), matches...)
	// To simplify the loop below, add a sentinal: an empty match that is at
	// the end of the file.
	// TODO Determine if the sentinal helps.
	// TODO Determine if adding another sentinal to match the start
	// (i.e. BlockMatch{-1, -1, 0}) would help as well.
	matchesByA = append(matchesByA, BlockMatch{
		AIndex: aLineCount,
		BIndex: bLineCount,
		Length: 0,
	})

	// Reduce complexity by treating A as primary, achieved by swapping if
	// A is not primary, and swapping again before returning.
	if !aIsPrimary {
		SwapBlockMatches(matchesByA)
		aLineCount, bLineCount = bLineCount, aLineCount
	}
	SortBlockMatchesByAIndex(matchesByA)
	matchesByB := append([]BlockMatch(nil), matchesByA...)
	SortBlockMatchesByBIndex(matchesByB)

	// Create an index from BlockMatch to position in matchesByB, allowing
	// us to detect adjacency in B when processing matchesByA.
	matches2BOrder := make(map[BlockMatch]int)
	for n := range matchesByB {
		matches2BOrder[matchesByB[n]] = n
	}

	isAdjacentToPredecessor := func(aOrder, bOrder int) bool {
		if aOrder == 0 {
			return bOrder == 0
		}
		return matches2BOrder[matchesByA[aOrder-1]] == bOrder-1
	}

	// March through the BlockMatches
	// creating BlockPairs representing matches, moves, and mismatches
	// (insertions, deletions and conflicts).
	// We don't emit a non-match when a move is encountered (i.e. between
	// two sequential matches in matchesByA that are not sequential in
	// matchesByB).

	loA, loB := 0, 0
	hasMoves := false
	for aOrder := range matchesByA {
		ma := matchesByA[aOrder]
		bOrder := matches2BOrder[ma]
		isMove := true
		if isAdjacentToPredecessor(aOrder, bOrder) {
			isMove = false
			aGapLength := ma.AIndex - loA
			bGapLength := ma.BIndex - loB
			if aGapLength > 0 || bGapLength > 0 {
				pairs = append(pairs, BlockPair{
					AIndex:  loA,
					ALength: aGapLength,
					BIndex:  loB,
					BLength: bGapLength,
					IsMatch: false,
					IsMove:  false, // Not a move relative to its neighbors.
				})
			}
		} else {
			hasMoves = true
		}
		if ma.Length > 0 {  // Skip the sentinal.
			// Emit a match.
			pairs = append(pairs, BlockPair{
				AIndex:  ma.AIndex,
				ALength: ma.Length,
				BIndex:  ma.BIndex,
				BLength: ma.Length,
				IsMatch: true,
				IsMove:  isMove,
			})
			loA, loB = ma.AIndex + ma.Length, ma.BIndex + ma.Length
		}
	}

	// Determine if there are any lines from B missing in the pairs as a result
	// of treating A as primary.
	hasAGaps := false
	hasBGaps := false
	if hasMoves {
		unmatchedAs := make(map[int]bool)
		loA = 0
		for n := range pairs {
			for limit := pairs[n].AIndex; loA < limit; loA++ {
				unmatchedAs[loA] = true
			}
		}
		hasAGaps = len(unmatchedAs) > 0

		SortBlockPairsByBIndex(pairs)

		loA, loB = 0, 0
		var bInserts []BlockPair

		for n := range pairs {
			if loB < pairs[n].BIndex {
				if 




				if n == 0 {
					// TODO Could be that the block represents a move, and we should
					// really match up the block with the 

			
				bInserts = append(

				hasBGaps = true
				break
			}
			loB += pairs[n].BLength
		}

		



		if hasBGaps {
			// Sort the BlockPairs by B, and fill the gaps with Inserts (BlockPairs
			// with no corresponding lines in A).
			// TODO This is where we may want a heuristic about lines we'd like to keep
			// together (e.g. if line n is non-empty AND less indented than line n+1,
			// we commonly expect this to mean that line n+1 is somehow subordinate or
			// contained in something that started on n; we might also work backwards
			// on that, for example:
			//  1:  Lorem ipsum dolor sit amet, consectetur
			//  2:      adipiscing elit, sed do eiusmod
			//  3:    tempor incididunt ut labore et dolore magna
			//  4:  aliqua. Ut enim ad minim veniam, quis nostrud
			// Here we might infer that 1 is superior to 2 and 3, and that the pairs
			// 2-3, 3-4, and 1-4 have no such relationship.


		// TODO Implement gap filling for B
		// TODO Implement gap filling for A (aligning with B's gaps where possible)
		// TODO Determine how to mark the moves such that as little is considered
		// to have moved as possible (i.e. if moved a line from start to end, don't
		// want to make it appear that it was all the other lines that moved).
		/*
			      lowestBIndexAbove := make([]int, len(rawPairs))
			      lowestBIndexAbove[len(rawPairs)-1] = bLineCount
			      bLo := bLineCount
			      outOfOrderMatchCount := 0
			      outOfOrderLinesCount := 0
			      for n := len(rawPairs) - 1; n >= 0; n--
				      lowestBIndexAbove[n] = bLo
				      if bLo < rawPairs[n].BIndex
					      outOfOrderMatchCount++
					      outOfOrderLinesCount += rawPairs[n].BLength
				      else
					      bLo = rawPairs[n].BIndex

			      if outOfOrderMatchCount*2 < len(rawPairs) && outOfOrderLinesCount*2 < matchLinesCount
		*/






		SortBlockPairsByAIndex(pairs)
	}

	if hasGaps {
		// Sort the BlockPairs by B, and fill the gaps with Inserts (BlockPairs
		// with no corresponding lines in A).
		// TODO This is where we may want a heuristic about lines we'd like to keep
		// together (e.g. if line n is non-empty AND less indented than line n+1,
		// we commonly expect this to mean that line n+1 is somehow subordinate or
		// contained in something that started on n; we might also work backwards
		// on that, for example:
		//  1:  Lorem ipsum dolor sit amet, consectetur
		//  2:      adipiscing elit, sed do eiusmod
		//  3:    tempor incididunt ut labore et dolore magna
		//  4:  aliqua. Ut enim ad minim veniam, quis nostrud
		// Here we might infer that 1 is superior to 2 and 3, and that the pairs
		// 2-3, 3-4, and 1-4 have no such relationship.

		SortBlockPairsByBIndex(pairs)
		loB = 0

		// TODO Implement gap filling for B
		// TODO Implement gap filling for A (aligning with B's gaps where possible)
		// TODO Determine how to mark the moves such that as little is considered
		// to have moved as possible (i.e. if moved a line from start to end, don't
		// want to make it appear that it was all the other lines that moved).
		/*
			      lowestBIndexAbove := make([]int, len(rawPairs))
			      lowestBIndexAbove[len(rawPairs)-1] = bLineCount
			      bLo := bLineCount
			      outOfOrderMatchCount := 0
			      outOfOrderLinesCount := 0
			      for n := len(rawPairs) - 1; n >= 0; n--
				      lowestBIndexAbove[n] = bLo
				      if bLo < rawPairs[n].BIndex
					      outOfOrderMatchCount++
					      outOfOrderLinesCount += rawPairs[n].BLength
				      else
					      bLo = rawPairs[n].BIndex

			      if outOfOrderMatchCount*2 < len(rawPairs) && outOfOrderLinesCount*2 < matchLinesCount
		*/

		SortBlockPairsByAIndex(pairs)
	}

	if !aIsPrimary {
		SwapBlockPairs(pairs)
	}

	return
}

func FormatInterleaved(pairs []BlockPair, aIsPrimary bool, aFile, bFile *File,
	w io.Writer, printLineNumbers bool) error {
	pairs = append([]BlockPair(nil), pairs...)
	if aIsPrimary {
		SortBlockPairsByAIndex(pairs)
	} else {
		SortBlockPairsByBIndex(pairs)
	}
	maxDigits := DigitCount(maxInt(aFile.GetLineCount(), bFile.GetLineCount()))
	inMove := false
	for bn, bp := range pairs {
		glog.Infof("FormatInterleaved processing %d: %v", bp)
		if bn != 0 {
			fmt.Fprintln(w)
		}

		stoppingMove := false
		startingMove := false
		if inMove && bp.IsMove {
			// Reached the end of a move (TODO make sure to update if meaning of
			// IsMove changes, i.e. I figure out which set of blocks represents the
			// small move).
			stoppingMove = false
		} else if bp.IsMove {
			startingMove = true
			inMove = true
		}

		// Come up with a better visual display.
		var err error
		if startingMove {
			_, err = fmt.Fprintln(w, "Start of move +++++++++++++++++++++++++++++++++")
		} else if stoppingMove {
			_, err = fmt.Fprintln(w, "End of move -----------------------------------")
		}
		if err != nil {
			return err
		}

		printLines := func(f *File, start, length int, prefix rune) error {
			glog.Infof("printLines [%d, %d) of file %s", start, start+length, f.Name)

			for n := start; n < start+length; n++ {
				if printLineNumbers { // TODO Compute max width, use to right align.
					if _, err := fmt.Fprint(w, FormatLineNum(n+1, maxDigits), " "); err != nil {
						return err
					}
				}
				fmt.Fprint(w, string(prefix), "\t")
				if _, err := w.Write(f.GetLineBytes(n)); err != nil {
					return err
				}
			}
			return nil
		}

		formatStartAndLength := func(start, length int) string {
			if length == 1 {
				return fmt.Sprintf("%d", start)
			} else {
				return fmt.Sprintf("%d,%d", start, length)
			}
		}

		if bp.IsMatch {
			// TODO Maybe print line numbers, especially if in a move?
			if aIsPrimary {
				err = printLines(aFile, bp.AIndex, bp.ALength, '=')
			} else {
				err = printLines(bFile, bp.BIndex, bp.BLength, '=')
			}
			if err != nil {
				return err
			}
			continue
		}

		_, err = fmt.Fprint(w, "@@ -", formatStartAndLength(bp.AIndex+1, bp.ALength), " +",
			formatStartAndLength(bp.BIndex+1, bp.BLength), " @@\n")
		if err != nil {
			return err
		}
		if bp.ALength > 0 {
			err = printLines(aFile, bp.AIndex, bp.ALength, '-')
		}
		if bp.BLength > 0 {
			err = printLines(bFile, bp.BIndex, bp.BLength, '+')
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func FormatSideBySide(pairs []BlockPair, aIsPrimary bool, aFile, bFile *File,
	w io.Writer, displayWidth, spacesPerTab int,
	printLineNumbers, truncateLongLines bool) error {
	pairs = append([]BlockPair(nil), pairs...)
	if aIsPrimary {
		SortBlockPairsByAIndex(pairs)
	} else {
		SortBlockPairsByBIndex(pairs)
	}

	// TODO Calculate how much width we need for line numbers (based on number
	// of digits required for largest line number, 1-based, so that is number
	// of lines in the larger file)
	// TODO Calculate how much width we assign to each file, leaving room for
	// leading digits (might not be the same if we have an odd number of chars
	// available).
	// TODO Consider issue related to multibyte runes.

	return nil
}
