package dm

import (
	"fmt"
	"io"

	"github.com/golang/glog"
)

// Define BlockMatches U and V in matches to be "adjacent matches"
// when there exist integers i and j such that:
//      matchesByA[i] == U      matchesByA[i + 1] == V
//      matchesByB[j] == U      matchesByB[j + 1] == V
// We want to identify such adjacent matches because we can then be
// confident in emiting a conflict, insertion or deletion between
// two, rather than there being a likelihood that the gap between the
// two represents a move.

// TODO Experiment with how block COPIES are handled; Not sure these are
// remotely correct yet.

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

type matchesToPairsState struct {
	aFile, bFile             *File
	aLineCount, bLineCount   int
	matchesByA, matchesByB   []BlockMatch
	hasMoves                 bool
	unmatchedAs, unmatchedBs map[int]bool
	pairs                    []*BlockPair
}

// We assume that A is the primary file.  If not, then caller should take
// care of swapping before and after calling.

func InitMatchesToPairsState(aFile, bFile *File, matches []BlockMatch) *matchesToPairsState {
	p := &matchesToPairsState{
		aFile:       aFile,
		bFile:       bFile,
		aLineCount:  len(aFile.Lines),
		bLineCount:  len(bFile.Lines),
		matchesByA:  append([]BlockMatch(nil), matches...),
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
		AIndex: p.aLineCount,
		BIndex: p.bLineCount,
		Length: 0,
	})

	SortBlockMatchesByAIndex(p.matchesByA)
	p.matchesByB = append([]BlockMatch(nil), p.matchesByA...)
	SortBlockMatchesByBIndex(p.matchesByB)

	return p
}

func markAsIdenticalMatch(pair *BlockPair) {
	pair.IsMatch = true
	pair.IsNormalizedMatch = false
}

func markAsNormalizedMatch(pair *BlockPair) {
	pair.IsMatch = false
	pair.IsNormalizedMatch = true
}

func markAsMismatch(pair *BlockPair) {
	pair.IsMatch = false
	pair.IsNormalizedMatch = false
}

type gapFiller struct {
	converter          *matchesToPairsState
	loA, hiA, loB, hiB int
}

func (g *gapFiller) matchPrefix(equiv func(a, b LinePos) bool, finalizer func(pair *BlockPair)) int {
	aLines := g.converter.aFile.Lines
	bLines := g.converter.bFile.Lines
	limit := minInt(g.hiA-g.loA, g.hiB-g.loB)
	prefix := 0
	for ; prefix < limit; prefix++ {
		if !equiv(aLines[g.loA+prefix], bLines[g.loB+prefix]) {
			break
		}
	}
	if prefix > 0 {
		pair := &BlockPair{
			AIndex:  g.loA,
			ALength: prefix,
			BIndex:  g.loB,
			BLength: prefix,
		}
		finalizer(pair)
		glog.Infof("matchPrefix found: %v", *pair)
		g.converter.pairs = append(g.converter.pairs, pair)
		g.loA += prefix
		g.loB += prefix
	}
	return prefix
}

func (g *gapFiller) matchSuffix(equiv func(a, b LinePos) bool, finalizer func(pair *BlockPair)) int {
	aLines := g.converter.aFile.Lines
	bLines := g.converter.bFile.Lines
	limit := minInt(g.hiA-g.loA, g.hiB-g.loB)
	suffix := 1
	for ; suffix <= limit; suffix++ {
		if !equiv(aLines[g.hiA-suffix], bLines[g.hiB-suffix]) {
			break
		}
	}
	suffix--
	if suffix > 0 {
		pair := &BlockPair{
			AIndex:  g.hiA - suffix,
			ALength: suffix,
			BIndex:  g.hiB - suffix,
			BLength: suffix,
		}
		finalizer(pair)
		glog.Infof("matchSuffix found: %v", *pair)
		g.converter.pairs = append(g.converter.pairs, pair)
		g.hiA -= suffix
		g.hiB -= suffix
	}
	return suffix
}

type FillPhase int

const (
	PhaseByAWithGaps FillPhase = iota
	PhaseByBWithAGaps
	PhaseFinal
)

func exactMatch(a, b LinePos) bool {
	return a.Hash == b.Hash
}

func normalizedMatch(a, b LinePos) bool {
	return a.NormalizedHash == b.NormalizedHash
}

func (p *matchesToPairsState) fillGap(loA, hiA, loB, hiB int, phase FillPhase) {
	glog.Infof("fillGap %d->%d,  %d->%d,   %v", loA, hiA, loB, hiB, phase)

	g := gapFiller{p, loA, hiA, loB, hiB}

	if phase == PhaseByAWithGaps || phase == PhaseByBWithAGaps {
		// For now allowing normalized. May want to change that
		// later if I use Tichy's "string comparision with block moves" algorithm
		// applied to rare lines only, per Cohen's patience diff.
		g.matchPrefix(exactMatch, markAsIdenticalMatch)
		g.matchSuffix(exactMatch, markAsIdenticalMatch)
		g.matchPrefix(normalizedMatch, markAsNormalizedMatch)
		g.matchSuffix(normalizedMatch, markAsNormalizedMatch)
	}

	if (phase == PhaseByAWithGaps || phase == PhaseFinal || !p.hasMoves) && (g.hiA > g.loA || g.hiB > g.loB) {
		pair := &BlockPair{
			AIndex:            g.loA,
			ALength:           g.hiA - g.loA,
			BIndex:            g.loB,
			BLength:           g.hiB - g.loB,
			IsMatch:           false,
			IsMove:            false,
			IsNormalizedMatch: false,
		}
		glog.Infof("fillGap inserting: %v", *pair)
		p.pairs = append(p.pairs, pair)
	}
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
				p.fillGap(loA, ma.AIndex, loB, ma.BIndex, PhaseByAWithGaps)
			}
		} else {
			p.hasMoves = true
			for n := loA; n < ma.AIndex; n++ {
				p.unmatchedAs[n] = true
			}
		}
		// Emit an exact match.
		pair := &BlockPair{
			AIndex:  ma.AIndex,
			ALength: ma.Length,
			BIndex:  ma.BIndex,
			BLength: ma.Length,
			IsMatch: true,
			IsMove:  isMove,
		}
		glog.Infof("createPairsByA inserting: %v", *pair)
		p.pairs = append(p.pairs, pair)
		loA, loB = ma.AIndex+ma.Length, ma.BIndex+ma.Length
	}
}

func (p *matchesToPairsState) fillGapsByB() {
	SortBlockPairsByBIndex(p.pairs)
	p.combinePairs()

	// Find any gaps in B, attempt to fill them.
	for n, numPairs, loB := 0, len(p.pairs), 0; n < numPairs; n++ {
		pair := p.pairs[n]
		if loB < pair.BIndex {
			// There is a gap. Find the lines in A just before this pair
			// that are also unmatched.
			hiA := pair.AIndex
			loA := hiA - 1
			for ; loA >= 0 && p.unmatchedAs[loA]; loA-- {
				delete(p.unmatchedAs, loA)
			}
			loA++
			// TODO Consider skipping the common suffix matching here.
			p.fillGap(loA, hiA, loB, pair.BIndex, PhaseByBWithAGaps)
		}
		loB = pair.BIndex + pair.BLength
	}
}

func (p *matchesToPairsState) fillFinal() {
	// Find any gaps remaining in A and fill them. Depending upon presence
	// of sentinals here.
	SortBlockPairsByAIndex(p.pairs)
	p.combinePairs()
	loA := 0
	for n, numPairs := 1, len(p.pairs); n < numPairs; n++ {
		pair := p.pairs[n]
		if loA < pair.AIndex {
			p.pairs = append(p.pairs, &BlockPair{
				AIndex:  loA,
				ALength: pair.AIndex - loA,
				BIndex:  pair.BIndex,
				BLength: 0,
			})
			glog.Infof("fillFinal added missing A lines: %v",
				*p.pairs[len(p.pairs)-1])
		}
		loA = maxInt(loA, pair.AIndex+pair.ALength)
	}

	SortBlockPairsByBIndex(p.pairs)
	p.combinePairs()
	loB := 0
	for n, numPairs := 1, len(p.pairs); n < numPairs; n++ {
		pair := p.pairs[n]
		if loB < pair.BIndex {
			p.pairs = append(p.pairs, &BlockPair{
				AIndex:  pair.AIndex,
				ALength: 0,
				BIndex:  loB,
				BLength: pair.BIndex - loB,
			})
			glog.Infof("fillFinal added missing B lines: %v",
				*p.pairs[len(p.pairs)-1])
		}
		loB = maxInt(loB, pair.BIndex+pair.BLength)
	}
}

func BlockPairsAreSameType(p, o *BlockPair) bool {
	return p.IsMatch == o.IsMatch && p.IsNormalizedMatch == o.IsNormalizedMatch
}

func BlockPairsAreInOrder(p, o *BlockPair) bool {
	nextA := p.AIndex + p.ALength
	nextB := p.BIndex + p.BLength
	return nextA == o.AIndex && nextB == o.BIndex
}

func IsSentinal(p *BlockPair) bool {
	return p.AIndex < 0 || (p.ALength == 0 && p.BLength == 0)
}

// Sort by AIndex or BIndex before calling combinePairs.
func (p *matchesToPairsState) combinePairs() {
	// For each pair of consecutive BlockPairs, if they can be combined,
	// combine them into the first of them.
	u, v, limit := 0, 1, len(p.pairs)
	for v < limit {
		j, k := p.pairs[u], p.pairs[v]
		if BlockPairsAreSameType(j, k) && BlockPairsAreInOrder(j, k) && !IsSentinal(j) && !IsSentinal(k) {
			glog.Infof("Combining BlockPairs:\n[%d]: %v\n[%d]: %v", u, *j, v, *k)
			j.ALength += k.ALength
			j.BLength += k.BLength
			p.pairs[v] = nil
		} else {
			u++
		}
		v++
	}
	glog.Infof("Removed %d BlockPairs", v-u-1)
	p.pairs = p.pairs[0 : u+1]
}

// Confused routine to create the blocks that we'll output, representing
// matches between files, inserted lines, dropped lines, and conflicts.
// Because I want to support block moves (and by extension block copies),
// I may have lines from one file appearing twice. To simplify things I'm
// treating one file as primary, where its lines will appear in the output
// exactly once and in order, and the other will be secondary, and its lines
// will appear out of order IFF there have been moves. Note that copies may
// cause the primary lines to appear twice in the other file, which will
// be treated as an insertion rather than a copy (at least so far).
func BlockMatchesToBlockPairs(aFile, bFile *File, matches []BlockMatch) (
	pairs []*BlockPair) {
	p := InitMatchesToPairsState(aFile, bFile, matches)
	p.createPairsByA()
	if p.hasMoves {
		glog.Infof("Filling gaps left by moves")
		p.fillGapsByB()
		p.fillFinal()
	} else {
		glog.Infof("Found no moves")
	}
	SortBlockPairsByAIndex(p.pairs)
	p.combinePairs()
	return p.pairs[1 : len(p.pairs)-1] // Remove sentinals
}

func FormatInterleaved(pairs []*BlockPair, aIsPrimary bool, aFile, bFile *File,
	w io.Writer, printLineNumbers bool) error {
	pairs = append([]*BlockPair(nil), pairs...)
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

func FormatSideBySide(pairs []*BlockPair, aIsPrimary bool, aFile, bFile *File,
	w io.Writer, displayWidth, spacesPerTab int,
	printLineNumbers, truncateLongLines bool) error {
	pairs = append([]*BlockPair(nil), pairs...)
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
