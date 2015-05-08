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

	for i, aGapFR := range aGapRanges {
		if aGapFR.Length() == 0 { continue }

		glog.Infof("Comparing gap %d in A to all gaps in B: aGapFR %#v", i, aGapFR)

		for j, bGapFR := range bGapRanges {
			if bGapFr.Length() == 0 { continue }
			glog.Infof("Comparing gap %d in A to gap %d in B: bGapFR %#v", i, j, bGapFR)


		}
	}

	return
}

