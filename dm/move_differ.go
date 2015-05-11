package dm

import (
	"github.com/golang/glog"
	"sort"
)

//func compareTwoRanges

//func PerformLCS(fileRangePair FileRangePair, config DifferencerConfig, sf SimilarityFactors) *lcsOfFileRangePair {

type MoveCandidate2 struct {
	aGapIndex, bGapIndex int
	gapFRP               FileRangePair
	originalBGapRange    FileRange
	lcsData              *lcsOfFileRangePair
	moveScore            float64
}

func (p *MoveCandidate2) AExtent() int {
	return p.lcsData.AExtent()
}
func (p *MoveCandidate2) BExtent() int {
	return p.lcsData.BExtent()
}

var extentCurve = MakeSymmetricLogisticFunction(0, 1, 0.5, 1.5)
var distanceCurve = MakeSymmetricLogisticFunction(-100, 100, 0.5, 1.5)

// We want to order the candidates by various factors, not strictly ordered,
// so we compute a score for each candidate.
// * Higher LCS similarity score is better, but this can be due to lots of
//   common lines, depending on the SimilarityFactors.
// * Similar A and B extents is good, but they don't have to be exact.
// * Proximity to the original location is preferred, but not essential.
func (p *MoveCandidate2) SetScore() {
	if p.moveScore != 0 {
		return
	}

	aExtent, bExtent := p.AExtent(), p.BExtent()
	glog.V(1).Infof("MoveCandidate2.SetScore: lcsScore=%v,  aExtent=%v,  bExtent=%v",
		p.lcsData.lcsScore, aExtent, bExtent)

	var lesserExtent float64
	if aExtent < bExtent {
		lesserExtent = float64(aExtent)
	} else {
		lesserExtent = float64(bExtent)
	}
	extentScore := extentCurve.Compute(lesserExtent / float64(aExtent+bExtent))

	glog.V(1).Infof("MoveCandidate2.SetScore: extentScore=%v", extentScore)

	var distance float64
	if p.aGapIndex < p.bGapIndex {
		// limitsInB are above originalBGapRange.
		// Least distance
		hi, lo := p.lcsData.limitsInB.Index1, p.originalBGapRange.BeyondIndex()
		d1 := hi - lo
		// Greatest distance
		hi, lo = p.lcsData.limitsInB.Index2, p.originalBGapRange.FirstIndex()
		d2 := hi - lo
		distance = float64(d1+d2) / 2
	} else /* p.aGapIndex > p.bGapIndex */ {
		// limitsInB are below originalBGapRange.
		// Least distance
		lo, hi := p.lcsData.limitsInB.Index2, p.originalBGapRange.FirstIndex()
		d1 := hi - lo
		// Greatest distance
		lo, hi = p.lcsData.limitsInB.Index1, p.originalBGapRange.BeyondIndex()
		d2 := hi - lo
		distance = float64(d1+d2) / 2
	}

	normalizedDistance := distance * 100 / float64(p.originalBGapRange.File().LineCount())
	distanceScore := distanceCurve.Compute(-normalizedDistance)

	glog.V(1).Infof("MoveCandidate2.SetScore: distance=%v,  normalizedDistance=%v,  distanceScore=%v", distance, normalizedDistance, distanceScore)

	p.moveScore = float64(p.lcsData.lcsScore) * extentScore * distanceScore
	glog.Infof("MoveCandidate2.SetScore gap %d in A vs. gap %d in B, moveScore = %v",
		p.aGapIndex, p.bGapIndex, p.moveScore)
}

//// Find common prefixes and suffixes between the pairs.
//func (p *MoveCandidate2) ExtendPairs(state *diffState) {
//
//}

type MoveCandidate2s []*MoveCandidate2

func (v MoveCandidate2s) Len() int {
	return len(v)
}
func (v MoveCandidate2s) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}
func (v MoveCandidate2s) Less(i, j int) bool {
	return v[i].moveScore < v[j].moveScore
}
func (v MoveCandidate2s) SetScores() {
	for _, mc := range v {
		mc.SetScore()
	}
}

func PerformMoveDetectionInGaps(
	frp FileRangePair, blockPairs BlockPairs, config DifferencerConfig,
	sf SimilarityFactors) (
	outputBlockPairs BlockPairs) {
	defer glog.Flush()

	glog.Infof("PerformMoveDetectionInGaps: %d blockPairs input", len(blockPairs))

	aGapRanges, bGapRanges := FindGapsInRangePair(frp, blockPairs)

	glog.Infof("len(aGapRanges) == %d", len(aGapRanges))
	glog.Infof("len(bGapRanges) == %d", len(bGapRanges))

//	multipleCandidates = false
	var allMoveCandidates MoveCandidate2s
	for i, aGapFR := range aGapRanges {
		glog.Infof("Comparing gap %d in A to all gaps in B: aGapFR %s", i, aGapFR)

		if aGapFR.Length() == 0 {
			glog.Info("Skipping gap with no A lines")
			continue
		}

		var theseMoveCandidates MoveCandidate2s

		for j, bGapFR := range bGapRanges {
			glog.Infof("Comparing gap %d in A to gap %d in B: bGapFR %s", i, j, bGapFR)
			if bGapFR.Length() == 0 {
				glog.Info("Skipping gap with no B lines")
				continue
			}
			if i == j {
				glog.Info("Skipping corresponding gap in B (can't be a move if matches)")
				continue
			}

			gapFRP := frp.BaseFilePair().MakeFileRangePair(aGapFR, bGapFR)

			lcsData := PerformLCS(gapFRP, config, sf)
			if lcsData == nil {
				glog.V(1).Info("Found no alignment between these gaps")
				continue
			}
			glog.Info("Found an alignment between these gaps")
			mc := &MoveCandidate2{
				aGapIndex:         i,
				bGapIndex:         j,
				gapFRP:            gapFRP,
				lcsData:           lcsData,
				originalBGapRange: bGapRanges[i],
			}
			theseMoveCandidates = append(theseMoveCandidates, mc)
		}

		if len(theseMoveCandidates) == 0 {
			continue
		}
		allMoveCandidates = append(allMoveCandidates, theseMoveCandidates...)
		//		if len(theseMoveCandidates) > 1 {
		//			// Move the "best" to the beginning.
		//			theseMoveCandidates.SetScores()
		//			sort.Sort(sort.Reverse(theseMoveCandidates))
		//			multipleCandidates = true
		//		}
		//		allMoveCandidates = append(allMoveCandidates, theseMoveCandidates[0])
	}

	if len(allMoveCandidates) > 0 {
		allMoveCandidates.SetScores()
		sort.Sort(sort.Reverse(allMoveCandidates))
	}

	matchedBIndices := MakeIntervalSet()
	insertBIndices := func(pairs BlockPairs) {
		for _, pair := range pairs {
			matchedBIndices.InsertInterval(pair.BIndex, pair.BBeyond())
		}
	}
	containsAnyBIndices := func(pairs BlockPairs) bool {
		for _, pair := range pairs {
			if matchedBIndices.ContainsSome(pair.BIndex, pair.BBeyond()) {
				return true
			}
		}
		return false
	}

	var newBlockPairs BlockPairs
	for n, mc := range allMoveCandidates {
		glog.V(1).Infof("Considering move candidate #%d", n)
		if containsAnyBIndices(mc.lcsData.lcsPairs) {
			glog.Infof("Move candidate #%d overlaps with another move candidate", n)
			continue
		}
		pairs := mc.lcsData.lcsPairs
		pairs.AssignMoveId()
		insertBIndices(pairs)
		newBlockPairs = append(newBlockPairs, pairs...)
	}

	outputBlockPairs = append(BlockPairs(nil), blockPairs...)
	outputBlockPairs = append(outputBlockPairs, newBlockPairs...)
	SortBlockPairsByAIndex(outputBlockPairs)
	return
}
