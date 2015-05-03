package dm

import (
	"github.com/golang/glog"
)

func PerformDiff2(aFile, bFile *File, config DifferencerConfig) (pairs []*BlockPair) {
	defer glog.Flush()
	rootRangePair := MakeFullFileRangePair(aFile, bFile)
	if rootRangePair.RangesAreSame(true /* onlyExactMatches */) {
		glog.Info("PerformDiff2: files are identical")
		return nil
	} else if rootRangePair.RangesAreSame(false /* onlyExactMatches */) {
		glog.Info("PerformDiff2: files are identical after normalization")
		// TODO Calculate indentation changes.
		panic("TODO Calculate indentation changes. Make BlockPairs")
	} else if !rootRangePair.BothAreNotEmpty() {
		// One is empty (not both, else the ranges would be the same).
		if aFile.LineCount() > 0 {
			pair := &BlockPair{
				AIndex: 0,
				ALength: aFile.LineCount(),
				BIndex: 0,
				BLength: 0,
			}
			pairs = append(pairs, pair)
		} else {
			pair := &BlockPair{
				AIndex: 0,
				ALength: 0,
				BIndex: 0,
				BLength: bFile.LineCount(),
			}
			pairs = append(pairs, pair)
		}
		return
	}

	rootDiffer := MakeSimpleDiffer(rootRangePair)
	maxRareOccurrences := uint8(MinInt(255, config.MaxRareLineOccurrencesInFile))

	// Phase 1: Match ends

	if config.MatchEnds {
		rootDiffer.SetMiddleToGap(config.OmitProbablyCommonLines,
															config.MatchNormalizedEnds, maxRareOccurrences)
		if !rootDiffer.MiddleRangesAreNotEmpty() {
			// One of the middle ranges is empty (not both, because otherwise we'd
			// have discovered that above).
			panic("TODO Create BlockPair for middle, for shared ends, sort and return")
		}
	} else {
		rootDiffer.SetMiddleToBase()
	}

	// Phase 2: LCS alignment.

	normSim := MaxFloat32(0, MinFloat32(1, float32(config.LcsNormalizedSimilarity)))
	halfDelta := (1 - normSim) / 2
	sf := SimilarityFactors{
		MaxRareOccurrences: maxRareOccurrences,
		ExactRare: 1,
		NormalizedRare: normSim,
		ExactNonRare: 1 - halfDelta,
		NormalizedNonRare: MaxFloat32(0, normSim - halfDelta),
	}
	if !config.AlignNormalizedLines {
		sf.NormalizedRare = 0
		sf.NormalizedNonRare = 0
	}
	if config.AlignRareLines {
		sf.ExactRare = 0
		sf.ExactNonRare = 0
	}

	rootDiffer.ComputeWeightedLCSOfMiddle(s)

	// Phase 3: If have done rare and/or exact match alignment only, then we may
	// have gaps that consist solely of normalized and/or non-rare matches, and
	// don't need to consider them later. 


	}


	rootDiffer.BaseRangesAreNotEmpty()

	//
	//
	//// Note that common prefix may overlap, as when comparing these two strings
	//// for common prefix and suffix: "ababababababa" and "ababa".
	//// Returns true if fully consumed.
	//func (p *simpleDiffer) MeasureCommonEnds(onlyExactMatches bool, maxRareOccurrences uint8) (rangesSame bool) {
	//	return p.baseRangePair.MeasureCommonEnds(onlyExactMatches, maxRareOccurrences)
	//}
	//

	return nil
}
