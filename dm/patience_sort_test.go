package dm

import (
	"sort"
	"testing"
)

type IntInt [][]int

func (s IntInt) Len() int      { return len(s) }
func (s IntInt) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s IntInt) Less(i, j int) bool {
	if len(s[i]) != len(s[j]) {
		panic("wrong length")
	}
	si := s[i]
	sj := s[j]
	for n := range si {
		if si[n] != sj[n] {
			return si[n] < sj[n]
		}
	}
	return false
}

func (s IntInt) AssertEq(o IntInt, t *testing.T) {
	if len(s) != len(o) {
		t.Errorf("lengths are not equal: %d != %d", len(s), len(o))
		return
	}
	for n := range s {
		sn := s[n]
		on := o[n]
		if len(sn) != len(on) {
			t.Errorf("[%d] lengths are not equal: %d != %d", n, len(sn), len(on))
			return
		}
		for m := range sn {
			if sn[m] != on[m] {
				t.Errorf("[%d, %d] values are not equal: %d != %d", n, m, sn[m], on[m])
			}
		}
	}
}

func GetSortedPatienceSortResults(input... int) (results IntInt) {
	ch := PatienceSort(input)
	for lis := range ch {
		results = append(results, lis)
	}
	sort.Sort(results)
	return
}

func TestPatienceSort(t *testing.T) {
	results := GetSortedPatienceSortResults(9, 13, 7, 12, 2, 1, 4, 6, 5, 8, 3, 11, 10)
	results.AssertEq(IntInt{
		[]int{1, 4, 5, 8, 10},
		[]int{1, 4, 5, 8, 11},
		[]int{1, 4, 6, 8, 10},
		[]int{1, 4, 6, 8, 11},
		[]int{2, 4, 5, 8, 10},
		[]int{2, 4, 5, 8, 11},
		[]int{2, 4, 6, 8, 10},
		[]int{2, 4, 6, 8, 11},
	}, t)
	results = GetSortedPatienceSortResults(10, 9, 8)
	results.AssertEq(IntInt{
		[]int{8},
		[]int{9},
		[]int{10},
	}, t)
}

func TestPatienceSort0(t *testing.T) {
	results := GetSortedPatienceSortResults()
	if nil != results {
		t.Errorf("Expected no results, not: %v", results)
	}
	empty := make([]int, 0, 1)
	results = GetSortedPatienceSortResults(empty...)
	if nil != results {
		t.Errorf("Expected no results, not: %v", results)
	}
}
