package dm

import (
	"fmt"
	"io"

	"github.com/golang/glog"
)

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

	// Define BlockMatches U and V in matches to be "adjacent matches"
	// when there exist integers i and j such that:
	//      matchesByA[i] == U      matchesByA[i + 1] == V
	//      matchesByB[j] == U      matchesByB[j + 1] == V
	// We want to identify such adjacent matches because we can then be
	// confident in emiting a conflict, insertion or deletion between
	// two, rather than there being a likelihood that the gap between the
	// two represents a move.

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
		if ma.Length > 0 {
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

//	if !hasMoves && (loA < aLineCount || loB < bLineCount) {
//		pairs = append(pairs, BlockPair{
//			AIndex:  loA,
//			ALength: aLineCount - loA,
//			BIndex:  loB,
//			BLength: bLineCount - loB,
//			IsMatch: false,
//			IsMove:  false, // Not a move relative to its neighbors.
//		})
//	}

	// Determine if there are any lines from B missing in the pairs as a result
	// of treating A as primary.
	hasGaps := false
	if hasMoves {
		SortBlockPairsByBIndex(pairs)
		loB = 0

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
