package dm
// Compute the Longest Common Sequence of Atoms, where Atom is an interface
// allowing for comparison, while hiding the underlying "meaning".

import (
	"github.com/golang/glog"
)

type IndexPair struct {
	AIndex, BIndex int
}

// Dynamic programming solution to produce a weighted LCS of two "strings"
// A and B of length aLength and bLength, respectively.
// getSimilarity must return a value between 0 and 1, inclusive.
func WeightedLCS(aLength, bLength int, getSimilarity func(aIndex, bIndex int) float32) (
		result []IndexPair) {

	// Convention: the first index of the table (related to positions in A) is
	// called the up-down index (up is towards 0), and the second index is called
	// the right-left index (left is towards 0). 
	table := make([][]float32, aLength + 1)
	for i := range table {
		table[i] = make([]float32, bLength + 1)
	}

	for aIndex := 0; aIndex < aLength; aIndex++ {
		for bIndex := 0; bIndex < bLength; bIndex++ {
			// How similar are A[aIndex] and B[bIndex]?
			similarity := getSimilarity(aIndex, bIndex)

			// Compute the value to be placed in table[aIndex+1][bIndex+1], which
			// is the weighted length of the LCS if it the strings A and B were of
			// length aIndex+1 and bIndex+1.  Because the similarity is not just
			// zero or one, we need to figure compute the maximum of 3 possible
			// values.
			maxNonSimilar := maxFloat32(table[aIndex][bIndex+1], table[aIndex+1][bIndex])

			if similarity > 0 {
				weightedLength := similarity + table[aIndex][bIndex]
				if weightedLength <= maxNonSimilar {
					glog.Infof("[%d, %d] weightedLength=%v, maxNonSimilar=%v",
						aIndex, bIndex, weightedLength, maxNonSimilar)
				}
				table[aIndex+1][bIndex+1] = maxFloat32(weightedLength, maxNonSimilar)
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
		}	else {
			a--
			b--
			result = append(result, IndexPair{a, b})
		}
	}

	// Reverse the result.
	for i, j := 0, len(result) - 1; i < j; {
		result[i], result[j] = result[j], result[i]
		i++
		j--
	}

	return result
}
