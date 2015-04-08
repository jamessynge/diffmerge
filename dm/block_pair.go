package dm

import (
	"log"
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
	log.Printf("BlockMatchesToBlockPairs")
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
		return matches2BOrder[matchesByA[aOrder - 1]] == bOrder - 1
	}

	// March through the BlockMatches
	// creating BlockPairs representing matches, moves, and mismatches
	// (insertions, deletions and conflicts).
	// We don't emit a non-match when a move is encountered (i.e. between
	// two sequential matches in matchesByA that are not sequential in
	// matchesByB).

	loA, loB := 0, 0
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
					BIndex: loB,
					BLength: bGapLength,
					IsMatch: false,
					IsMove: false,  // Not a move relative to its neighbors.
				})
			}
		}
		// Emit a match.
		pairs = append(pairs, BlockPair{
			AIndex: ma.AIndex,
			ALength: ma.Length,
			BIndex: ma.BIndex,
			BLength: ma.Length,
			IsMatch: true,
			IsMove: isMove,
		})
		loA, loB = ma.AIndex, ma.BIndex
	}

	// Sort the BlockPairs by B, and fill the gaps with Inserts (BlockPairs
	// with no corresponding lines in A).
	SortBlockPairsByBIndex(pairs)
	loB = 0




		ma, mb := matchesByA[n], matchesByB[n]
		if ma == mb {
			// Alignment maintained.
			// Completely unsure what I want for BIndex in this BlockPair, in
			// the face of moves. Thought needed.
			nextB := maxInt(loB, minInt(ma.BIndex, mb.BIndex))
			if loA < ma.AIndex || loB < nextB {
				// There is a gap below to be expressed.
				pairs = append(pairs, BlockPair{
					AIndex:  loA,
					ALength: ma.AIndex - loA,
					BIndex: loB,
					BLength: nextB - loB,
					IsMatch: false,
					IsMove: false,
				})
				loA, loB = ma.AIndex, nextB
			}
		
		
		
		}
	
	
	
	}
	





	// Produce BlockPairs indicating a match for each BlockMatch.
	var pairsA, pairsB []BlockPair
	matchLinesCount := 0
	for n := range matches {
		matchLinesCount += matches[n].Length
		pairsA = append(pairsA, BlockPair{
			AIndex:  matches[n].AIndex,
			ALength: matches[n].Length,
			BIndex:  matches[n].BIndex,
			BLength: matches[n].Length,
			IsMatch: true,
		})
	}

	// Reduce complexity by treating A as primary, achieved by swapping if
	// A is not primary, and then again before returning.
	if !aIsPrimary {
		SwapBlockPairs(pairsA)
		aLineCount, bLineCount = bLineCount, aLineCount
	}

	// Use two arrays of pairs, sorted by the two indices.
	SortBlockPairsByAIndex(pairsA)
	pairsB := append([]BlockPair(nil), pairsA...)

	// Which pairs represent moves, relative to the primary index? Those where
	// the secondary index isn't ascending.
	// TODO Not sure how to handle a huge move (such as first line to last line);
	// want to consider that a move of a single line, rather than as a move of
	// all the other lines.
	lowestBIndexAbove := make([]int, len(rawPairs))
	lowestBIndexAbove[len(rawPairs)-1] = bLineCount
	bLo := bLineCount
	outOfOrderMatchCount := 0
	outOfOrderLinesCount := 0
	for n := len(rawPairs) - 1; n >= 0; n-- {
		lowestBIndexAbove[n] = bLo
		if bLo < rawPairs[n].BIndex {
			outOfOrderMatchCount++
			outOfOrderLinesCount += rawPairs[n].BLength
		} else {
			bLo = rawPairs[n].BIndex
		}
	}

	if outOfOrderMatchCount*2 < len(rawPairs) && outOfOrderLinesCount*2 < matchLinesCount {
		//

	}

	/*
	   for range rawPairs {
	     i := len(rawPairs) -n
	     if n == 0 { continue }
	     if rawPairs[n-1].BIndex > rawPairs[n].BIndex {
	       rawPairs[n-1].IsMove = true
	     }
	   }
	*/

	for n := range rawPairs {
		if n == 0 {
			continue
		}
		if rawPairs[n-1].BIndex > rawPairs[n].BIndex {
			rawPairs[n-1].IsMove = true
		}
	}

	// Sort by the primary index and fill any gaps in pairs, w.r.t. the primary index.
	aLo := 0
	var aPairs []BlockPair
	for n := range rawPairs {
		if rawPairs[n].AIndex > aLo {
			// There is a gap in the primary key.  Insert a mismatch.
			// TODO Need a way to find this entry later when dealing with gaps
			// in the secondary key.
			aPairs = append(aPairs, BlockPair{
				AIndex:  aLo,
				ALength: rawPairs[n].AIndex - aLo,
				BIndex:  -1,
				BLength: -1,
				IsMatch: true,
			})
		}
	}

	// Produce BlockPairs indicating a match for each BlockMatch.

	/*

	   // Represents a pairing of ranges in files A and B, primarily for output,
	   // as we can produce different pairings based on which file we consider
	   // primary (i.e. in the face of block moves we may print A in order, but
	   // B out of order).
	   type BlockPair struct {
	   	AIndex, ALength int
	   	BIndex, BLength int
	   	IsMatch         bool
	   }
	*/

	if !aIsPrimary {
		SwapBlockPairs(pairs)
	}

	return
}
