package dm

import (
	"github.com/golang/glog"
)

func PerformMoveDetectionInGaps(
		frp FileRangePair, blockPairs []*BlockPair, config DifferencerConfig) (
		outputBlockPairs []*BlockPair) {
	defer glog.Flush()

	glog.Infof("PerformMoveDetectionInGaps: %d blockPairs input", len(blockPairs))

	aGapRanges, bGapRanges := FindGapsInRangePair(frp, blockPairs)

	glog.Infof("len(aGapRanges) == %d", len(aGapRanges))
	glog.Infof("len(bGapRanges) == %d", len(bGapRanges))

	for n, aGapFR := range(aGapRanges) {
		glog.Infof("Comparing gap in A to all gaps in B: aGapFR %#v", aGapFR)

		if aGapFR.Length() == 0 { continue }





		bGapFR := bGapRanges[n]

		glog.Infof("aGapFR: %#v", aGapFR)
		glog.Infof("bGapFR: %#v", bGapFR)

		glog.Info("TODO develop criteria for deciding if a mismatch is likely an edit")
	}

	return
}

