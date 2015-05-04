package dm

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
)

type SharedEndsKey struct {
	OnlyExactMatches   bool
	MaxRareOccurrences uint8
}

type SharedEndsData struct {
	SharedEndsKey
	// If the lines in the range are equal, or equal after normalization
	// (approximately equal), then one or both of these booleans are true,
	// and the prefix and suffix lengths are 0.
	RangesAreEqual, RangesAreApproximatelyEqual bool

	// The FileRangePair which was measured to produce this.
	Source FileRangePair

	NonRarePrefixLength, NonRareSuffixLength    int
	RarePrefixLength, RareSuffixLength          int
}

func (p *SharedEndsData) PrefixAndSuffixOverlap(rareEndsOnly bool)  bool {
	limit, combinedLength := MinInt(p.Source.ALength(), p.Source.BLength()), 0
	if rareEndsOnly {
		combinedLength = p.RarePrefixLength + p.RareSuffixLength
	} else {
		combinedLength = p.NonRarePrefixLength + p.NonRareSuffixLength
	}
	return limit < combinedLength
}

func (p *SharedEndsData) HasRarePrefixOrSuffix() bool {
	return p.RarePrefixLength > 0 || p.RareSuffixLength > 0
}

func (p *SharedEndsData) HasPrefixOrSuffix() bool {
	return p.NonRarePrefixLength > 0 || p.NonRareSuffixLength > 0
}

func (p *SharedEndsData) GetPrefixAndSuffixLengths(rareEndsOnly bool) (prefixLength, suffixLength int) {
	if p.PrefixAndSuffixOverlap(rareEndsOnly) {
		// Caller needs to guide the process more directly.
		cfg := spew.Config
		cfg.MaxDepth = 2
		glog.Fatal("GetPrefixAndSuffixLengths: prefix and suffix overlap;\n",
							 cfg.Sdump(p))
	}
	if rareEndsOnly {
		return p.RarePrefixLength, p.RareSuffixLength
	} else {
		return p.NonRarePrefixLength, p.NonRareSuffixLength
	}
	return
}
