package dm

import (
	"github.com/golang/glog"
)

type MatchCommonXFunc func(aRange, bRange FileRange, normalized bool) (
	aRest, bRest FileRange, bp *BlockPair)

// Find all lines at the start that are the same (the common prefix).
// Produces at most one match; if normalized==true, then that match may
// contain both full and normalized line matches (separating those will
// need to happen elsewhere).
func MatchCommonPrefix(aRange, bRange FileRange, normalized bool) (
	aRemaining, bRemaining FileRange, commonPrefix *BlockPair) {

	aLineCount, bLineCount := aRange.LineCount(), bRange.LineCount()
	limit := MinInt(aLineCount, bLineCount)

	glog.Info("MatchCommonPrefix: ", aLineCount, " lines from A, ",
		bLineCount, " lines from B, limit of ", limit,
		chooseString(normalized, "; comparing normalized lines", ""))

	length := 0
	for ; length < limit; length++ {
		ah := aRange.LineHashAtOffset(length, normalized)
		bh := bRange.LineHashAtOffset(length, normalized)
		if ah != bh {
			break
		}
	}
	if length == 0 {
		return aRange, bRange, nil
	}
	commonPrefix = &BlockPair{
		AIndex:            aRange.LinePosAtOffset(0).Index,
		ALength:           length,
		BIndex:            bRange.LinePosAtOffset(0).Index,
		BLength:           length,
		IsMatch:           !normalized,
		IsNormalizedMatch: normalized,
	}

	glog.Infof("MatchCommonPrefix: emit BlockPair: %v", *commonPrefix)

	if length < aLineCount {
		aRemaining = aRange.MakeSubRange(length, aLineCount-length)
	}
	if length < bLineCount {
		bRemaining = bRange.MakeSubRange(length, bLineCount-length)
	}
	return
}

// Find all lines at the end that are the same (the common suffix).
// Produces at most one match; if normalized==true, then that match may
// contain both full and normalized line matches (separating those will
// need to happen elsewhere).
func MatchCommonSuffix(aRange, bRange FileRange, normalized bool) (
	aRemaining, bRemaining FileRange, commonSuffix *BlockPair) {

	aLineCount, bLineCount := aRange.LineCount(), bRange.LineCount()
	limit := MinInt(aLineCount, bLineCount)

	glog.Info("MatchCommonSuffix: ", aLineCount, " lines from A, ",
		bLineCount, " lines from B, limit of ", limit,
		chooseString(normalized, "; comparing normalized lines", ""))

	length := 0
	aOffset, bOffset := aLineCount-1, bLineCount-1
	for length < limit {
		ah := aRange.LineHashAtOffset(aOffset, normalized)
		bh := bRange.LineHashAtOffset(bOffset, normalized)
		if ah != bh {
			break
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
		AIndex:            aRange.LinePosAtOffset(aOffset).Index,
		ALength:           length,
		BIndex:            bRange.LinePosAtOffset(bOffset).Index,
		BLength:           length,
		IsMatch:           !normalized,
		IsNormalizedMatch: normalized,
	}

	glog.Infof("MatchCommonSuffix: emit BlockPair: %v", *commonSuffix)

	if length < aLineCount {
		aRemaining = aRange.MakeSubRange(0, length)
	}
	if length < bLineCount {
		bRemaining = bRange.MakeSubRange(0, length)
	}
	return
}

func MatchCommonEnds(aRange, bRange FileRange, prefix, suffix, normalized bool) (
	aRest, bRest FileRange, pairs []*BlockPair) {

	if FileRangeIsEmpty(aRange) || FileRangeIsEmpty(bRange) {
		// glog.Warning("Wasted call to MatchCommonEnds with emtpy range(s)")
		return aRange, bRange, nil
	}

	glog.Infof("MatchCommonEnds A lines: %d; B lines: %d; prefix: %v; suffix: %v; normalized: %v",
		aRange.LineCount(), bRange.LineCount(),
		prefix, suffix, normalized)

	tryMatch := func(matcher MatchCommonXFunc) (done bool) {
		var bp *BlockPair
		aRest, bRest, bp = matcher(aRange, bRange, normalized)
		if bp == nil {
			return false // Assuming here that both ranges are non-empty.
		}
		pairs = append(pairs, bp)
		if aRest == nil {
			glog.Infof("MatchCommonEnds: matched ALL %d lines of aRange", aRange.LineCount())
			done = true
		} else {
			before, after := aRange.LineCount(), aRest.LineCount()
			if after != before {
				glog.Infof("MatchCommonEnds: matched %d lines of %d, leaving %d  (aRange)",
					before-after, before, after)
			}
		}
		if bRest == nil {
			glog.Infof("MatchCommonEnds: matched ALL %d lines of bRange", bRange.LineCount())
			done = true
		} else {
			before, after := bRange.LineCount(), bRest.LineCount()
			if after != before {
				glog.Infof("MatchCommonEnds: matched %d lines of %d, leaving %d  (bRange)",
					before-after, before, after)
			}
		}
		return done
	}
	if prefix {
		if tryMatch(MatchCommonPrefix) || !suffix {
			// All done.
			return
		}
		aRange, bRange = aRest, bRest
	}
	if suffix {
		tryMatch(MatchCommonSuffix)
	}
	return
}
