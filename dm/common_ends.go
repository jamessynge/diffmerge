package dm

import (
	"github.com/golang/glog"
)

// Find all lines at the start that are the same (the common prefix).
// Produces at most one match; if normalized==true, then that match may
// contain both full and normalized line matches (separating those will
// need to happen elsewhere).
func MatchCommonPrefix(aRange, bRange FileRange, normalized bool) (
	aRemaining, bRemaining FileRange, commonPrefix *BlockPair) {

	aLineCount, bLineCount := aRange.GetLineCount(), bRange.GetLineCount()
	limit := minInt(aLineCount, bLineCount)
	isContiguous := aRange.IsContiguous() && bRange.IsContiguous()

	glog.Info("MatchCommonPrefix: ", aLineCount, " lines from A, ",
		bLineCount, " lines from B, limit of ", limit,
		chooseString(normalized, "; comparing normalized lines", ""),
		chooseString(isContiguous, "; lines may not be contiguous", ""))

	length := 0
	var ai, bi int
	for ; length < limit; length++ {
		ah := aRange.GetLineHashRelative(length, normalized)
		bh := bRange.GetLineHashRelative(length, normalized)
		if ah != bh {
			break
		}
		if !isContiguous {
			aLP := aRange.GetLinePosRelative(length)
			bLP := bRange.GetLinePosRelative(length)
			if length > 0 {
				if aLP.Index != ai+1 || bLP.Index != bi+1 {
					// The lines aren't immediately adjacent.
					break
				}
				ai++
				bi++
			} else {
				ai, bi = aLP.Index, bLP.Index
			}
		}
	}
	if length == 0 {
		return aRange, bRange, nil
	}
	commonPrefix = &BlockPair{
		AIndex:            aRange.GetLinePosRelative(0).Index,
		ALength:           length,
		BIndex:            bRange.GetLinePosRelative(0).Index,
		BLength:           length,
		IsMatch:           !normalized,
		IsNormalizedMatch: normalized,
	}

	glog.Infof("MatchCommonPrefix: emit BlockPair: %v", *commonPrefix)

	if length < aLineCount {
		aRemaining = aRange.GetSubRange(length, aLineCount-length)
	}
	if length < bLineCount {
		bRemaining = bRange.GetSubRange(length, bLineCount-length)
	}
	return
}

// Find all lines at the end that are the same (the common suffix).
// Produces at most one match; if normalized==true, then that match may
// contain both full and normalized line matches (separating those will
// need to happen elsewhere).
func MatchCommonSuffix(aRange, bRange FileRange, normalized bool) (
	aRemaining, bRemaining FileRange, commonSuffix *BlockPair) {

	aLineCount, bLineCount := aRange.GetLineCount(), bRange.GetLineCount()
	limit := minInt(aLineCount, bLineCount)
	isContiguous := aRange.IsContiguous() && bRange.IsContiguous()

	glog.Info("MatchCommonSuffix: ", aLineCount, " lines from A, ",
		bLineCount, " lines from B, limit of ", limit,
		chooseString(normalized, "; comparing normalized lines", ""),
		chooseString(isContiguous, "; lines may not be contiguous", ""))

	length := 0
	aOffset, bOffset := aLineCount-1, bLineCount-1
	var ai, bi int
	for length < limit {
		ah := aRange.GetLineHashRelative(aOffset, normalized)
		bh := bRange.GetLineHashRelative(bOffset, normalized)
		if ah != bh {
			break
		}
		if !isContiguous {
			aLP := aRange.GetLinePosRelative(aOffset)
			bLP := bRange.GetLinePosRelative(bOffset)
			if length > 0 {
				if aLP.Index != ai-1 || bLP.Index != bi-1 {
					// The lines aren't immediately adjacent.
					break
				}
				ai--
				bi--
			} else {
				ai, bi = aLP.Index, bLP.Index
			}
		}
		length++
		aOffset--
		bOffset--
	}
	if length == 0 {
		return aRange, bRange, nil
	}
	aOffset++
	bOffset++
	commonSuffix = &BlockPair{
		AIndex:            aRange.GetLinePosRelative(aOffset).Index,
		ALength:           length,
		BIndex:            bRange.GetLinePosRelative(bOffset).Index,
		BLength:           length,
		IsMatch:           !normalized,
		IsNormalizedMatch: normalized,
	}

	glog.Infof("MatchCommonSuffix: emit BlockPair: %v", *commonSuffix)

	if length < aLineCount {
		aRemaining = aRange.GetSubRange(0, length)
	}
	if length < bLineCount {
		bRemaining = bRange.GetSubRange(0, length)
	}
	return
}
