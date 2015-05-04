package dm

import (
	"github.com/golang/glog"
)

// Dynamic programming solution to produce a weighted LCS of two "strings"
// A and B of length aLength and bLength, respectively.
// getSimilarity must return a value between 0 and 1, inclusive.
// TODO In some senarios we prefer the shortest of several LCS (i.e. they
// have the same number of symbols in the string, but one may have fewer other
// symbols in it, insertions/deletions, between the first match and the last
// than other LCS candidates).  Would be best if we could produce all candidates,
// along with their weights, and then score them. Basically the short-coming of
// the basic LCS approach is that it doesn't consider any measure other than
// number of symbols (and here the weight of pairs of symbols), but doesn't
// have any other objective function.
// TODO Lots of opportunity here for (well known) optimizations.
func WeightedLCS(aLength, bLength int, getSimilarity func(aIndex, bIndex int) float32) (
	result []IndexPair, score float32) {

	// Convention: the first index of the table (related to positions in A) is
	// called the up-down index (up is towards 0), and the second index is called
	// the right-left index (left is towards 0).
	table := make([][]float32, aLength+1)
	for i := range table {
		table[i] = make([]float32, bLength+1)
	}

	for aIndex := 0; aIndex < aLength; aIndex++ {
		for bIndex := 0; bIndex < bLength; bIndex++ {
			// How similar are A[aIndex] and B[bIndex]?
			similarity := getSimilarity(aIndex, bIndex)

			// Compute the value to be placed in table[aIndex+1][bIndex+1], which
			// is the weighted length of the LCS if the strings A and B were of
			// length aIndex+1 and bIndex+1.  Because the similarity is not just
			// zero or one, we need to compute the maximum of 3 possible values.
			maxNonSimilar := MaxFloat32(table[aIndex][bIndex+1], table[aIndex+1][bIndex])

			if similarity > 0 {
				weightedLength := similarity + table[aIndex][bIndex]
				if weightedLength <= maxNonSimilar {
					glog.Infof("[%d, %d] weightedLength=%v, maxNonSimilar=%v",
						aIndex, bIndex, weightedLength, maxNonSimilar)
				}
				table[aIndex+1][bIndex+1] = MaxFloat32(weightedLength, maxNonSimilar)
			} else {
				table[aIndex+1][bIndex+1] = maxNonSimilar
			}
		}
	}

	// Backtrack to generate the result (in reverse order).
	// Starting at the end of the table (maximum indices), follow the path of
	// the largest values back towards the origin (minimum indices).
	// There may be multiple results possible, but we're just producing one.
	for a, b := aLength, bLength; a != 0 && b != 0; {
		if table[a][b] == table[a-1][b] {
			a--
		} else if table[a][b] == table[a][b-1] {
			b--
		} else {
			a--
			b--
			result = append(result, IndexPair{a, b})
		}
	}

	// Reverse the result.
	for i, j := 0, len(result)-1; i < j; {
		result[i], result[j] = result[j], result[i]
		i++
		j--
	}

	return result, table[aLength][bLength]
}

type SimilarityFactors struct {
	ExactRare          float32
	NormalizedRare     float32
	ExactNonRare       float32
	NormalizedNonRare  float32
	MaxRareOccurrences uint8
}

func (s *SimilarityFactors) SimilarityOfRangeLines(pair FileRangePair, aOffset, bOffset int) float32 {
	equal, approx, rare := pair.CompareLines(aOffset, bOffset, s.MaxRareOccurrences)
	if equal {
		if rare {
			return s.ExactRare
		} else {
			return s.ExactNonRare
		}
	}
	if approx {
		if rare {
			return s.NormalizedRare
		} else {
			return s.NormalizedNonRare
		}
	}
	return 0
}

func WeightedLCSOffsetsOfRangePair(pair FileRangePair, sf SimilarityFactors) (lcsOffsetPairs []IndexPair, score float32) {
	aLength, bLength := pair.ALength(), pair.BLength()
	if aLength == 0 || bLength == 0 { return }
	computeSimilarity := func(aOffset, bOffset int) float32 {
		return sf.SimilarityOfRangeLines(pair, aOffset, bOffset)
	}
	glog.Infof("WeightedLCSOfRangePair: %s", pair.BriefDebugString())
	lcsOffsetPairs, score = WeightedLCS(aLength, bLength, computeSimilarity)
	glog.Infof("WeightedLCSOfRangePair: LCS length == %d,   LCS score: %v", len(lcsOffsetPairs), score)
	return
}

func WeightedLCSBlockPairsOfRangePair(
		pair FileRangePair, sf SimilarityFactors) (blockPairs []*BlockPair, score float32) {
	lcsOffsetPairs, score := WeightedLCSOffsetsOfRangePair(pair, sf)
	matchedNormalizedLines := sf.NormalizedNonRare > 0 || sf.NormalizedRare > 0
	blockPairs = MatchingRangePairOffsetsToBlockPairs(pair, lcsOffsetPairs, matchedNormalizedLines, sf.MaxRareOccurrences)
	return blockPairs, score
}
