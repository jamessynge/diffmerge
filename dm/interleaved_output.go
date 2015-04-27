package dm

import (
	"fmt"
	"io"

	"github.com/golang/glog"
)

func FormatInterleaved(pairs []*BlockPair, aIsPrimary bool, aFile, bFile *File,
	w io.Writer, printLineNumbers bool) error {
	pairs = append([]*BlockPair(nil), pairs...)
	if aIsPrimary {
		SortBlockPairsByAIndex(pairs)
	} else {
		SortBlockPairsByBIndex(pairs)
	}
	maxDigits := DigitCount(MaxInt(aFile.LineCount(), bFile.LineCount()))
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
