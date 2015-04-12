package dm

import ()

// Given two files, and a ranges of lines in each file, find rare lines in those
// two ranges that are equally rare in both of the ranges.
// normalizedMatch == true to use the hashes of the normalized lines.
// sameCount == true to require the rare lines to appear the same number of
// times in each range.
// maxCount is the maximum number of times a hash may appear in the range
// and still be considered rare; maxCount==1 is the Patience Diff approach.
func FindRareLinesInRanges(aRange, bRange FileRange,
	normalizedMatch, sameCount bool, maxCount int) (aRareLines, bRareLines []LinePos) {
	var aHashPositions, bHashPositions HashPositions
	if normalizedMatch {
		aHashPositions = aRange.GetNormalizedHashPositions()
		bHashPositions = bRange.GetNormalizedHashPositions()
	} else {
		aHashPositions = aRange.GetHashPositions()
		bHashPositions = bRange.GetHashPositions()
	}

	rareHashes := make(map[uint32]bool)
	for hash, ap := range aHashPositions {
		al := len(ap)
		if !(1 <= al && al <= maxCount) {
			continue
		}
		if bp, ok := bHashPositions[hash]; ok {
			bl := len(bp)
			if sameCount {
				if al != bl {
					continue
				}
				// They're both equally rare.
			} else {
				if !(1 <= bl && bl <= maxCount) {
					continue
				}
				// They're both rare enough.
			}
			rareHashes[hash] = true
		}
	}

	var getter func(lp LinePos) uint32
	if normalizedMatch {
		getter = GetLPNormalizedHash
	} else {
		getter = GetLPHash
	}
	selector := func(lp LinePos) bool {
		return rareHashes[getter(lp)]
	}
	aRareLines = aRange.Select(selector)
	bRareLines = bRange.Select(selector)
	return
}
