package dm

import (
	"log"
	"sort"
)

// BlockPairByAIndex implements sort.Interface for []BlockPair based on
// the AIndex field, then BIndex.
type BlockPairByAIndex []BlockPair

func (a BlockPairByAIndex) Len() int           { return len(a) }
func (a BlockPairByAIndex) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BlockPairByAIndex) Less(i, j int) bool {
  if a[i].AIndex != a[j].AIndex {
    return a[i].AIndex < a[j].AIndex
  }
  return a[i].BIndex < a[j].BIndex
}

func SortBlockPairsByAIndex(a []BlockPair) {
  sort.Sort(BlockPairByAIndex(a))
}

func SwapBlockPairs(a []BlockPair) {
  for n := range a {
    a[n].AIndex, a[n].BIndex = a[n].BIndex, a[n].AIndex
    a[n].ALength, a[n].BLength = a[n].BLength, a[n].ALength
  }
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
		log.Printf("BlockMatchesToBlockPairs")
  // Produce BlockPairs indicating a match for each BlockMatch.
  var rawPairs []BlockPair
  matchLinesCount := 0
  for n := range matches {
    matchLinesCount += matches[n].Length
    rawPairs = append(pairs, BlockPair{
        AIndex: matches[n].AIndex,
        ALength: matches[n].Length,
        BIndex: matches[n].BIndex,
        BLength: matches[n].Length,
        IsMatch: true,
    })
  }

  // Reduce complexity by treating A as primary by swapping if necessary.
  if !aIsPrimary {
    SwapBlockPairs(rawPairs)
    aLineCount, bLineCount = bLineCount, aLineCount
  }
	SortBlockPairsByAIndex(rawPairs)

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

  if outOfOrderMatchCount * 2 < len(rawPairs) && outOfOrderLinesCount * 2 < matchLinesCount {
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
    if n == 0 { continue }
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
      aPairs = append(aPairs,  BlockPair{
          AIndex: aLo,
          ALength: rawPairs[n].AIndex - aLo,
          BIndex: -1,
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



