package dm

import (
	"github.com/golang/glog"
)

func PerformDiff2(aFile, bFile *File, config DifferencerConfig) (pairs []*BlockPair) {
	defer glog.Flush()
	if aFile.LineCount() == 0 {
		if bFile.LineCount() == 0 {
			return nil // They're the same.
		}
		pair := &BlockPair{
			AIndex:  0,
			ALength: 0,
			BIndex:  0,
			BLength: bFile.LineCount(),
		}
		return append(pairs, pair)
	} else if bFile.LineCount() == 0 {
		pair := &BlockPair{
			AIndex:  0,
			ALength: aFile.LineCount(),
			BIndex:  0,
			BLength: 0,
		}
		return append(pairs, pair)
	}
	filePair := MakeFilePair(aFile, bFile)
	rootRangePair := filePair.FullFileRangePair()

	// Phase 1: Match ends

	var mase *middleAndSharedEnds
	middleRangePair := rootRangePair
	if config.MatchEnds {
		mase = FindMiddleAndSharedEnds(rootRangePair, config)
		if mase != nil {
			if mase.sharedEndsData.RangesAreEqual {
				return
			} else if mase.sharedEndsData.RangesAreApproximatelyEqual {
				glog.Info("PerformDiff2: files are identical after normalization")
				// TODO Calculate indentation changes.
				panic("TODO Calculate indentation changes. Make BlockPairs")
			}
			middleRangePair = mase.middleRangePair
		}
	}

	// Phase 2: LCS alignment.

	maxRareOccurrences := uint8(MaxInt(1, MinInt(255, config.MaxRareLineOccurrencesInFile)))
	normSim := MaxFloat32(0, MinFloat32(1, float32(config.LcsNormalizedSimilarity)))
	halfDelta := (1 - normSim) / 2
	sf := SimilarityFactors{
		MaxRareOccurrences: maxRareOccurrences,
		ExactRare:          1,
		NormalizedRare:     normSim,
		ExactNonRare:       1 - halfDelta,
		NormalizedNonRare:  MaxFloat32(0, normSim-halfDelta),
	}
	if !config.AlignNormalizedLines {
		sf.NormalizedRare = 0
		sf.NormalizedNonRare = 0
	}
	if config.AlignRareLines {
		sf.ExactNonRare = 0
		sf.NormalizedNonRare = 0
	}

	lcsData := PerformLCS(middleRangePair, config, sf)

	if glog.V(1) {
		glog.Info("PerformLCS produced the following")
		var debugPairs BlockPairs
		if mase != nil {
			debugPairs = append(debugPairs, mase.sharedPrefixPairs...)
		}
		if lcsData != nil {
			debugPairs = append(debugPairs, lcsData.lcsPairs...)
		}
		if mase != nil {
			debugPairs = append(debugPairs, mase.sharedSuffixPairs...)
		}
		glogSideBySide(aFile, bFile, debugPairs, false, nil)
	}

	// TODO Phase 3: Small edit detection (nearly match gap in A with corresponding
	// gap in B)

	var middleBlockPairs BlockPairs
	if lcsData != nil {
		middleBlockPairs = append(middleBlockPairs, lcsData.lcsPairs...)
	}
	// NOTE: Not sure I like this idea, as we'll go from having only matches
	// to having matches and non-matches in middleBlockPairs.
	//   	middleBlockPairs = PerformSmallEditDetectionInGaps(middleRangePair, middleBlockPairs, config)

	// Phase 4a: move detection (match a gap in A with some gap(s) in B)

	numMatchedLines, _ := middleBlockPairs.CountLinesInPairs()
	middleBlockPairs = PerformMoveDetectionInGaps(middleRangePair, middleBlockPairs, config, sf)
	newNumMatchedLines, _ := middleBlockPairs.CountLinesInPairs()
	glog.Infof("Found %d moved or copied lines", newNumMatchedLines-numMatchedLines)

	var allMatches BlockPairs
	if mase != nil {
		allMatches = append(allMatches, mase.sharedPrefixPairs...)
	}
	allMatches = append(allMatches, middleBlockPairs...)
	if mase != nil {
		allMatches = append(allMatches, mase.sharedSuffixPairs...)
	}

	if glog.V(1) {
		glog.Info("PerformMoveDetectionInGaps produced the following")
		glogSideBySide(aFile, bFile, allMatches, false, nil)
	}

	// Phase 4b: Extend matches forward, then backwards. Do before copy or edit
	// detection.

	allMatches = ExtendMatchesForward(filePair, allMatches)
	allMatches = ExtendMatchesBackward(filePair, allMatches)

	// TODO Split mixed matches.

	// Combine matches.
	SortBlockPairsByBIndex(allMatches)
	allMatches = CombineBlockPairs(allMatches)

	allPairs := FillRemainingBGapsWithMismatches(filePair, allMatches)

	return allPairs

	// TODO Phase 5: copy detection (match a gap in B with similar size region anywhere in file A)

	//	rootDiffer.BaseRangesAreNotEmpty()

	//
	//
	//// Note that common prefix may overlap, as when comparing these two strings
	//// for common prefix and suffix: "ababababababa" and "ababa".
	//// Returns true if fully consumed.
	//func (p *simpleDiffer) MeasureCommonEnds(onlyExactMatches bool, maxRareOccurrences uint8) (rangesSame bool) {
	//	return p.baseRangePair.MeasureCommonEnds(onlyExactMatches, maxRareOccurrences)
	//}
	//

	// Phase 6: common & normalized matches (grow unique line matches forward, then backwards).

}

func ExtendMatchesForward(filePair FilePair, inputPairs BlockPairs) (outputPairs BlockPairs) {
	matchedALines := AIndexBlockPairsToIntervalSet(
		inputPairs, SelectAllBlockPairs)
	matchedBLines := BIndexBlockPairsToIntervalSet(
		inputPairs, SelectAllBlockPairs)

	SortBlockPairsByBIndex(inputPairs)

	if glog.V(1) {
		glog.Info("ExtendMatchesForward input:")
		glogSideBySide(filePair.AFile(), filePair.BFile(), inputPairs, false, nil)
	}

	for _, oldPair := range inputPairs {
		glog.V(1).Infof("ExtendMatchesForward considering BlockPair %v", *oldPair)
		aIndex, bIndex := oldPair.ABeyond(), oldPair.BBeyond()
		var newPair *BlockPair
		for {
			glog.V(1).Infof("ExtendMatchesForward considering indices %d and %d", aIndex, bIndex)
			if aIndex >= filePair.ALength() || bIndex >= filePair.BLength() {
				glog.V(1).Info("ExtendMatchesForward: beyond EOF")
				break
			}
			if matchedALines.Contains(aIndex) || matchedBLines.Contains(bIndex) {
				glog.V(1).Info("ExtendMatchesForward: line contained")
				break
			}
			equal, approx, _ := filePair.CompareFileLines(aIndex, bIndex, 0)
			if !(equal || approx) {
				break
			}
			matchedALines.InsertInterval(aIndex, aIndex+1)
			matchedBLines.InsertInterval(bIndex, bIndex+1)
			if newPair != nil && newPair.IsMatch == equal {
				// Extend newPair.
				glog.V(1).Info("Extending BlockPair forward")
				newPair.ALength++
				newPair.BLength++
			} else {
				// Create a new pair.
				newPair = &BlockPair{
					AIndex:            aIndex,
					ALength:           1,
					BIndex:            bIndex,
					BLength:           1,
					IsMatch:           equal,
					IsNormalizedMatch: !equal,
				}
				glog.V(1).Infof("ExtendMatchesForward matching up %d and %d", aIndex, bIndex)
				outputPairs = append(outputPairs, newPair)
			}
			aIndex++
			bIndex++
		}
	}

	if len(outputPairs) == 0 {
		glog.V(1).Info("ExtendMatchesForward found no new matches")
		return inputPairs
	}

	if glog.V(1) {
		glog.Info("ExtendMatchesForward generated the following")
		glogSideBySide(filePair.AFile(), filePair.BFile(), outputPairs, false, nil)
	}

	outputPairs = append(outputPairs, inputPairs...)

	if glog.V(1) {
		glog.Info("ExtendMatchesForward produced the following")
		glogSideBySide(filePair.AFile(), filePair.BFile(), outputPairs, false, nil)
	}

	return outputPairs
}

func ExtendMatchesBackward(filePair FilePair, inputPairs BlockPairs) (outputPairs BlockPairs) {
	matchedALines := AIndexBlockPairsToIntervalSet(
		inputPairs, SelectAllBlockPairs)
	matchedBLines := BIndexBlockPairsToIntervalSet(
		inputPairs, SelectAllBlockPairs)

	SortBlockPairsByBIndex(inputPairs)
	for _, oldPair := range inputPairs {
		aIndex, bIndex := oldPair.AIndex, oldPair.BIndex
		var newPair *BlockPair
		for {
			if aIndex <= 0 || bIndex <= 0 {
				break
			}
			aIndex--
			bIndex--
			if matchedALines.Contains(aIndex) || matchedBLines.Contains(bIndex) {
				break
			}
			equal, approx, _ := filePair.CompareFileLines(aIndex, bIndex, 0)
			if !(equal || approx) {
				break
			}
			matchedALines.InsertInterval(aIndex, aIndex+1)
			matchedBLines.InsertInterval(bIndex, bIndex+1)
			if newPair != nil && newPair.IsMatch == equal {
				// Extend newPair.
				glog.V(1).Info("Extending BlockPair backward")
				newPair.AIndex--
				newPair.BIndex--
				newPair.ALength++
				newPair.BLength++
			} else {
				// Create a new pair.
				newPair = &BlockPair{
					AIndex:            aIndex,
					ALength:           1,
					BIndex:            bIndex,
					BLength:           1,
					IsMatch:           equal,
					IsNormalizedMatch: !equal,
				}
				glog.V(1).Infof("ExtendMatchesBackward matching up %d and %d", aIndex, bIndex)
				outputPairs = append(outputPairs, newPair)
			}
		}
	}

	if len(outputPairs) == 0 {
		glog.V(1).Info("ExtendMatchesBackward found no new matches")
		return inputPairs
	}

	outputPairs = append(outputPairs, inputPairs...)

	if glog.V(1) {
		glog.Info("ExtendMatchesBackward produced the following")
		glogSideBySide(filePair.AFile(), filePair.BFile(), outputPairs, false, nil)
	}

	return outputPairs
}
