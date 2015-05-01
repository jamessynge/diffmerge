package dm

import (
	"github.com/golang/glog"
)

func PerformDiff2(aFile, bFile *File, config DifferencerConfig) (pairs []*BlockPair) {
	defer glog.Flush()

	rootRangePair := MakeFullFileRangePair(aFile, bFile)
	rootDiffer := MakeSimpleDiffer(rootRangePair)
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

