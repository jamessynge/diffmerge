package dm

import (
	"fmt"
	"io"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

var _ = spew.Dump

// Define BlockMatches U and V in matches to be "adjacent matches"
// when there exist integers i and j such that:
//      matchesByA[i] == U      matchesByA[i + 1] == V
//      matchesByB[j] == U      matchesByB[j + 1] == V
// We want to identify such adjacent matches because we can then be
// confident in emiting a conflict, insertion or deletion between
// two, rather than there being a likelihood that the gap between the
// two represents a move.

func BlockPairsAreInOrder(p, o *BlockPair) bool {
	nextA := p.AIndex + p.ALength
	nextB := p.BIndex + p.BLength
	return nextA == o.AIndex && nextB == o.BIndex
}

// TODO Experiment with how block COPIES are handled; Not sure these are
// remotely correct yet.

func BlockPairsAreSameType(p, o *BlockPair) bool {
	return p.IsMatch == o.IsMatch && p.IsNormalizedMatch == o.IsNormalizedMatch
}

func IsSentinal(p *BlockPair) bool {
	return p.AIndex < 0 || (p.ALength == 0 && p.BLength == 0)
}

func (p *BlockPair) markAsIdenticalMatch() {
	p.IsMatch = true
	p.IsNormalizedMatch = false
}

func (p *BlockPair) markAsNormalizedMatch() {
	p.IsMatch = false
	p.IsNormalizedMatch = true
}

func (p *BlockPair) markAsMismatch() {
	p.IsMatch = false
	p.IsNormalizedMatch = false
}

func (p *BlockPair) ABeyond() int {
	return p.AIndex + p.ALength
}

func (p *BlockPair) BBeyond() int {
	return p.BIndex + p.BLength
}

func (p *BlockPair) IsSentinal() bool {
	return p.AIndex < 0 || (p.ALength == 0 && p.BLength == 0)
}

// Sort by AIndex or BIndex before calling CombineBlockPairs.
func CombineBlockPairs(sortedInput []*BlockPair) (output []*BlockPair) {
	if glog.V(1) {
		glog.Info("CombineBlockPairs entry")
		for n, pair := range sortedInput {
			glog.Infof("CombineBlockPairs sortedInput[%d] = %v", n, pair)
		}
	}

	output = append(output, sortedInput...)
	// For each pair of consecutive BlockPairs, if they can be combined,
	// combine them into the first of them.
	u, v, limit, removed := 0, 1, len(output), 0
	for v < limit {
		j, k := output[u], output[v]

		glog.Infof("CombineBlockPairs output[u=%d] = %v", u, j)
		glog.Infof("CombineBlockPairs output[v=%d] = %v", v, k)

		if BlockPairsAreSameType(j, k) && BlockPairsAreInOrder(j, k) && !IsSentinal(j) && !IsSentinal(k) {
			glog.Infof("Combining BlockPairs:\n[%d]: %v\n[%d]: %v", u, *j, v, *k)
			j.ALength += k.ALength
			j.BLength += k.BLength
			output[v] = nil
			removed++
		} else {
			// BlockPairs can't be combined.
			output[u+1] = k
			u++
		}
		v++
	}
	glog.Infof("Removed %d (= %d) BlockPairs", v-u-1, removed)

	output = output[0 : u+1]

	if glog.V(1) {
		glog.Info("CombineBlockPairs exit")
		for n, pair := range output {
			glog.Infof("CombineBlockPairs output[%d] = %v", n, pair)
		}
	}

	return output
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
		glog.V(3).Infof("FormatInterleaved processing %d: %v", bp)
		if bn != 0 {
			fmt.Fprintln(w)
		}

		stoppingMove := false
		startingMove := false
		if inMove && !bp.IsMove {
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
			glog.V(3).Infof("printLines [%d, %d) of file %s", start, start+length, f.Name)

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

