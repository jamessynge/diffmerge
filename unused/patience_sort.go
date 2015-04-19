package unused

import ()

// Specialization of patience sorting for (distinct) integers, rather than
// arbitrary items with some abstract comparison method.
// Input is a slice of integers in some order, output is one or more
// longest increasing subsequences of the original slice.

//type LISSource interface {
//	// Returns the next longest increasing subsequences, or an empty slice.
//	NextLIS() []int
//}

type patienceSortPiles struct {
	// Piles (each a slice) as defined by the Patience Sorting algorithm, where
	// a card (integer) may be placed on a pile (appended to a slice) only
	// if the top card (last value appended) is of lower value. If there is no
	// such pile, the card must be used to start a new pile (i.e. append a new
	// slice with just that integer to the slice of piles).
	piles [][]int

	// For determining a longest increasing subsequences, we need to know which
	// card was on top of pile N when adding a card to pile N+1, thus we record
	// how many cards
	// were in pile N, minus 1, when we added the kth card to pile N+1. In particular,
	// we append len(pile[N]) - 1 to the N+1'th backpointers slice.
	backPointers [][]int
}

// Sends longest increasing subsequences of a permutation
// of integers to a channel.  Intended for use as a goroutine.
func generateLISesFromPiles(p *patienceSortPiles, ch chan<- []int) {
	if p == nil || len(p.piles) == 0 {
		close(ch)
		return
	} else if len(p.piles) == 1 {
		for _, v := range p.piles[0] {
			ch <- []int{v}
		}
		close(ch)
		return
	}

	scratch := make([]int, len(p.piles))

	// Produce all LIS's ending with the entries in p.piles[pileIndex][0:top],
	// where those values must be < v; we know that p.piles[pileIndex][top] is
	// less than v, but that may also be true of other values placed in that
	// pile before top was added.
	var helper func(pileIndex, top, v int)
	helper = func(pileIndex, top, v int) {
		pile := p.piles[pileIndex]
		if pileIndex > 0 {
			bps := p.backPointers[pileIndex]
			u := pile[top]
			scratch[pileIndex] = u
			helper(pileIndex-1, bps[top], u)
			for top > 0 {
				top--
				u = pile[top]
				if u < v {
					scratch[pileIndex] = u
					helper(pileIndex-1, bps[top], u)
				} else {
					break
				}
			}
		} else if pileIndex == 0 {
			scratch[pileIndex] = pile[top]
			ch <- append([]int(nil), scratch...)
			for top > 0 {
				top--
				u := pile[top]
				if u < v {
					scratch[pileIndex] = u
					ch <- append([]int(nil), scratch...)
				} else {
					break
				}
			}
		}
	}

	// Produce all LIS's ending with the entries the last pile.
	lastPileIndex := len(p.piles) - 1
	lastPile := p.piles[lastPileIndex]
	lastBPs := p.backPointers[lastPileIndex]
	for top, v := range lastPile {
		scratch[lastPileIndex] = v
		helper(lastPileIndex-1, lastBPs[top], v)
	}

	close(ch)
}

// Performs patience sorting of the input, returns a channel from which all
// longest increasing subsequences of the input can be read.
func PatienceSort(input []int) <-chan []int {
	piles := make([][]int, 0, 16)
	backPointers := make([][]int, 0, 16)

	for _, v := range input {
		// Find the first pile whose last value is less than v.
		// Linear search for now, optimize if an issue.
		addTo := 0
		for ; addTo < len(piles); addTo++ {
			top := len(piles[addTo]) - 1
			if v < piles[addTo][top] {
				// We've found a pile we can place it on.
				break
			}
		}

		// Are we making a new pile?
		if addTo == len(piles) {
			// Yes, add an empty slice.
			piles = append(piles, nil)
			backPointers = append(backPointers, nil)
		}

		// Add the value to the selected pile.
		piles[addTo] = append(piles[addTo], v)

		// And record the index of the top value of the previous pile; we want to
		// know this because no value added to that pile later later can be a part
		// of an LIS involving v.
		if addTo > 0 {
			prevPileHeight := len(piles[addTo-1])
			backPointers[addTo] = append(backPointers[addTo], prevPileHeight-1)
		}
	}

	// Patience sorting is now complete. But we're interested in the longest
	// increasing sequence(s).

	output := make(chan []int)
	go generateLISesFromPiles(&patienceSortPiles{
		piles:        piles,
		backPointers: backPointers,
	},
		output)

	return output
}
