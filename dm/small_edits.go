package dm

import (
	"github.com/golang/glog"
)

func PerformSmallEditDetectionInGaps(
		frp FileRangePair, blockPairs []*BlockPair, config DifferencerConfig) (
		outputBlockPairs []*BlockPair) {
	defer glog.Flush()

	aGapRanges, bGapRanges := FindGapsInRangePair(frp, blockPairs)

	glog.Infof("len(aGapRanges) == %d", len(aGapRanges))
	glog.Infof("len(bGapRanges) == %d", len(bGapRanges))

	for n, aGapFR := range(aGapRanges) {
		bGapFR := bGapRanges[n]

		glog.Infof("aGapFR: %#v", aGapFR)
		glog.Infof("bGapFR: %#v", bGapFR)

	}

	return
}

