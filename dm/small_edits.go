package dm

import (
	"github.com/golang/glog"
)

func PerformSmallEditDetectionInGaps(
	frp FileRangePair, blockPairs BlockPairs, config DifferencerConfig) (
	outputBlockPairs BlockPairs) {
	defer glog.Flush()

	aGapRanges, bGapRanges := FindGapsInRangePair(frp, blockPairs)

	glog.Infof("len(aGapRanges) == %d", len(aGapRanges))
	glog.Infof("len(bGapRanges) == %d", len(bGapRanges))

	var newBlockPairs BlockPairs

	for n, aGapFR := range aGapRanges {
		bGapFR := bGapRanges[n]
		if glog.V(1) {
			glog.Infof("Comparing gap ranges #%d to each other\n", n)
			glog.Infof("aGapFR: %s", aGapFR)
			glog.Infof("bGapFR: %s", bGapFR)
		}
		if aGapFR.Length() == 0 || bGapFR.Length() == 0 {
			continue
		}
		glog.Info("TODO develop criteria for deciding if a mismatch is likely an edit\n")
	}

	outputBlockPairs = append(outputBlockPairs, blockPairs...)
	outputBlockPairs = append(outputBlockPairs, newBlockPairs...)
	SortBlockPairsByAIndex(outputBlockPairs)
	return
}
