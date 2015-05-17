package dm

import (
	"sort"

	"github.com/golang/glog"
)

// Simplistic interval set, which is supports determining if a point or
// interval is covered by the inserted intervals, but not which intervals;
// doesn't support removal.  Effectively therefore, an integer set.
type IntervalSet interface {
	InsertInterval(begin, beyond int)
	ContainsSome(begin, beyond int) bool
	ContainsAll(begin, beyond int) bool
	Contains(position int) bool

	// If position is in an interval, returns that interval. If position is
	// between two intervals, returns those two; if beyond last interval, returns
	// the last interval; if before first interval, returns the first interval.
	IntervalsAround(position int) (result []IndexPair, isContained bool)
}

func MakeIntervalSet() IntervalSet {
	return &intervalSet{}
}

//type RBColor bool
//const (
//    Red   RBColor = false  // iota doesn't work for non-integers. sigh
//    Black RBColor = true
//)
//type simpleIntervalSetNode struct {
//	begin, beyond int
//	left, right *simpleIntervalSetNode
//	color RBColor
//}

type intervalSet struct {
	s []IndexPair
}

// Finds lowest interval that an interval starting at begin could overlap
// or abut. Result is in the range [0, N], N==length of set.
func (p *intervalSet) searchForBegin(begin int) (index int) {
	return sort.Search(len(p.s), func(i int) bool {
		return begin <= p.s[i].Index2
	})
}

// Finds highest interval that an interval ending at beyond could overlap
// with or abut. Result is in the range [-1, N), N==length of set.
func (p *intervalSet) searchForBeyond(beyond int) (index int) {
	// Reversed search: search for the lowest interval that is above beyond
	// (i.e. that it could not overlap with), and subtract one.
	n := sort.Search(len(p.s), func(i int) bool {
		return beyond < p.s[i].Index1
	})
	return n - 1
}

func (p *intervalSet) insertIntervalAt(begin, beyond, index int) {
	p.s = append(p.s[:index],
		append([]IndexPair{IndexPair{begin, beyond}}, p.s[index:]...)...)
}

func (p *intervalSet) validate() {
	var beyond int
	for i, ip := range p.s {
		if ip.Index1 >= ip.Index2 {
			glog.Fatalf("Invalid interval #%d: %v", i, ip)
		}
		if i > 0 && beyond >= ip.Index1 {
			glog.Fatalf("Overlapping intervals #%d (%v) and #%d (%v): %v",
				i-1, p.s[i-1], i, ip)
		}
		beyond = ip.Index2
	}
}

func (p *intervalSet) InsertInterval(begin, beyond int) {
	if begin >= beyond {
		glog.Fatalf("Invalid interval: [%d, %d)", begin, beyond)
	}
	defer p.validate()
	loIndex := p.searchForBegin(begin)
	hiIndex := p.searchForBeyond(beyond)
	if hiIndex < loIndex {
		p.insertIntervalAt(begin, beyond, loIndex)
	} else if hiIndex == loIndex {
		ip := &p.s[loIndex]
		ip.Index1 = MinInt(ip.Index1, begin)
		ip.Index2 = MaxInt(ip.Index2, beyond)
	} else {
		// loIndex < hiIndex, therefore both are valid p.s indices.
		// Replace p.s[loIndex:hiIndex+1] with ip.
		p.s[loIndex] = IndexPair{MinInt(begin, p.s[loIndex].Index1),
			MaxInt(beyond, p.s[hiIndex].Index2)}
		p.s = append(p.s[:loIndex+1], p.s[hiIndex+1:]...)
	}
}

func (p *intervalSet) ContainsSome(begin, beyond int) bool {
	loIndex := p.searchForBegin(begin)
	hiIndex := p.searchForBeyond(beyond)
	if loIndex < hiIndex {
		// [begin, beyond) overlaps multiple intervals in p.
		return true
	}
	if loIndex > hiIndex {
		// [begin, beyond) overlaps zero intervals in p.
		return false
	}
	if beyond <= p.s[hiIndex].Index1 {
		return false
	}
	return true
}

func (p *intervalSet) ContainsAll(begin, beyond int) bool {
	loIndex := p.searchForBegin(begin)
	hiIndex := p.searchForBeyond(beyond)
	if loIndex != hiIndex {
		// There can't be a single interval containing [begin, beyond).
		return false
	}
	ip := &p.s[loIndex]
	return ip.Index1 <= begin && beyond <= ip.Index2
}

func (p *intervalSet) Contains(position int) bool {
	index := p.searchForBegin(position)
	if index >= len(p.s) {
		return false
	}
	return p.s[index].Index1 <= position && position < p.s[index].Index2
}

func (p *intervalSet) IntervalsAround(position int) (result []IndexPair, isContained bool) {
	length := len(p.s)
	if length == 0 {
		return
	}
	lastIndex := length - 1
	index := p.searchForBegin(position)
	if index >= lastIndex {
		result = append(result, p.s[lastIndex])
		return
	}
	if p.s[index].Index1 <= position {
		// By definition of searchForBegin, position <= p.s[index].Index2.
		result = append(result, p.s[index])
		if position == p.s[index].Index2 {
			result = append(result, p.s[index+1])
		} else {
			isContained = true
		}
		return
	}
	// position < p.s[index].Index1.
	if index > 0 {
		result = append(result, p.s[index-1])
	}
	result = append(result, p.s[index])
	return
}
