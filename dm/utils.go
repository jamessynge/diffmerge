package dm

import (
	"bytes"
	"fmt"
	"math"
	"sort"
)

//	// From https://blog.golang.org/errors-are-values
//	type errWriter struct {
//	    w   io.Writer
//	    err error
//	}
//	func (ew *errWriter) write(buf []byte) {
//	    if ew.err != nil {
//	        return
//	    }
//	    _, ew.err = ew.w.Write(buf)
//	}

func MinInt(i, j int) int {
	if i < j {
		return i
	} else {
		return j
	}
}

func MaxInt(i, j int) int {
	if i < j {
		return j
	} else {
		return i
	}
}

func MinFloat32(u, v float32) float32 {
	if u < v {
		return u
	} else {
		return v
	}
}

func MaxFloat32(u, v float32) float32 {
	if u < v {
		return v
	} else {
		return u
	}
}

func Max3Float32(u, v, w float32) float32 {
	if u < v {
		if v < w {
			return w
		}
		return v
	}
	if u < w {
		return w
	}
	return u
}

func chooseString(b bool, trueString, falseString string) string {
	if b {
		return trueString
	}
	return falseString
}

func GetLPHash(lp LinePos) uint32 {
	return lp.Hash
}

func GetLPNormalizedHash(lp LinePos) uint32 {
	return lp.NormalizedHash
}

func SelectHashGetter(normalized bool) func(lp LinePos) uint32 {
	if normalized {
		return GetLPNormalizedHash
	} else {
		return GetLPHash
	}
}

////////////////////////////////////////////////////////////////////////////////

// BlockMatchByAIndex implements sort.Interface for []BlockMatch based on
// the AIndex field, then BIndex.
type BlockMatchByAIndex []BlockMatch

func (a BlockMatchByAIndex) Len() int      { return len(a) }
func (a BlockMatchByAIndex) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BlockMatchByAIndex) Less(i, j int) bool {
	if a[i].AIndex != a[j].AIndex {
		return a[i].AIndex < a[j].AIndex
	}
	return a[i].BIndex < a[j].BIndex
}
func SortBlockMatchesByAIndex(a []BlockMatch) {
	sort.Sort(BlockMatchByAIndex(a))
}

// BlockMatchByBIndex implements sort.Interface for []BlockMatch based on
// the BIndex field, then AIndex.
type BlockMatchByBIndex []BlockMatch

func (a BlockMatchByBIndex) Len() int      { return len(a) }
func (a BlockMatchByBIndex) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BlockMatchByBIndex) Less(i, j int) bool {
	if a[i].BIndex != a[j].BIndex {
		return a[i].BIndex < a[j].BIndex
	}
	return a[i].AIndex < a[j].AIndex
}
func SortBlockMatchesByBIndex(a []BlockMatch) {
	sort.Sort(BlockMatchByBIndex(a))
}

////////////////////////////////////////////////////////////////////////////////

// IndexPairByIndex1 implements sort.Interface for []IndexPair based on
// the Index1 field, then Index2.
type IndexPairByIndex1 []IndexPair

func (a IndexPairByIndex1) Len() int      { return len(a) }
func (a IndexPairByIndex1) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a IndexPairByIndex1) Less(i, j int) bool {
	if a[i].Index1 != a[j].Index1 {
		return a[i].Index1 < a[j].Index1
	}
	return a[i].Index2 < a[j].Index2
}
func SortIndexPairsByIndex1(a []IndexPair) {
	sort.Sort(IndexPairByIndex1(a))
}

// IndexPairByIndex2 implements sort.Interface for []IndexPair based on
// the Index2 field, then Index1.
type IndexPairByIndex2 []IndexPair

func (a IndexPairByIndex2) Len() int      { return len(a) }
func (a IndexPairByIndex2) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a IndexPairByIndex2) Less(i, j int) bool {
	if a[i].Index2 != a[j].Index2 {
		return a[i].Index2 < a[j].Index2
	}
	return a[i].Index1 < a[j].Index1
}
func SortIndexPairsByIndex2(a []IndexPair) {
	sort.Sort(IndexPairByIndex2(a))
}

////////////////////////////////////////////////////////////////////////////////

// BlockPairByAIndex implements sort.Interface for []BlockPair based on
// the AIndex field, then BIndex.
type BlockPairByAIndex []*BlockPair

func (a BlockPairByAIndex) Len() int      { return len(a) }
func (a BlockPairByAIndex) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BlockPairByAIndex) Less(i, j int) bool {
	pi, pj := a[i], a[j]
	if pi.AIndex != pj.AIndex {
		return pi.AIndex < pj.AIndex
	}
	if pi.ALength != pj.ALength {
		return pi.ALength < pj.ALength
	}
	if pi.BIndex != pj.BIndex {
		return pi.BIndex < pj.BIndex
	}
	return pi.BIndex < pj.BIndex
}
func SortBlockPairsByAIndex(a []*BlockPair) {
	sort.Sort(BlockPairByAIndex(a))
}

// BlockPairByBIndex implements sort.Interface for []BlockPair based on
// the BIndex field, then AIndex.
type BlockPairByBIndex []*BlockPair

func (a BlockPairByBIndex) Len() int      { return len(a) }
func (a BlockPairByBIndex) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BlockPairByBIndex) Less(i, j int) bool {
	pi, pj := a[i], a[j]
	if pi.BIndex != pj.BIndex {
		return pi.BIndex < pj.BIndex
	}
	if pi.BLength != pj.BLength {
		return pi.BLength < pj.BLength
	}
	if pi.AIndex != pj.AIndex {
		return pi.AIndex < pj.AIndex
	}
	return pi.ALength < pj.ALength
}
func SortBlockPairsByBIndex(a []*BlockPair) {
	sort.Sort(BlockPairByBIndex(a))
}

////////////////////////////////////////////////////////////////////////////////

func DigitCount(i int) int {
	c := 0
	if i < 0 {
		c++
		i = -i
	} else if i == 0 {
		return 1
	}
	return c + int(math.Ceil(math.Log10(float64(i))))
}

func FormatLineNum(i, maxDigits int) string {
	return fmt.Sprintf("%*d", maxDigits, i)
}

func removeIndent(line []byte) []byte {
	line = bytes.TrimLeft(line, " \t")
	return line
}

func normalizeLine(line []byte) []byte {
	line = bytes.TrimSpace(line)
	// TODO Maybe collapse multiple spaces inside line, maybe remove all
	// spaces, maybe normalize case.
	return line
}

var wellKnownCommonLines map[string]bool

func init() {
	wellKnownCommonLines = make(map[string]bool)
	wellKnownCommonLines["//"] = true
	wellKnownCommonLines["/*"] = true
	wellKnownCommonLines["*"] = true
	wellKnownCommonLines["*/"] = true
	wellKnownCommonLines["#"] = true
	wellKnownCommonLines["("] = true
	wellKnownCommonLines[")"] = true
	wellKnownCommonLines["{"] = true
	wellKnownCommonLines["}"] = true
	wellKnownCommonLines["["] = true
	wellKnownCommonLines["]"] = true
	wellKnownCommonLines[""] = true
}

func ComputeIsProbablyCommon(normalizedLine []byte) bool {
	if len(normalizedLine) >= 3 {
		return false
	}
	return wellKnownCommonLines[string(normalizedLine)]
}

func computeNumRareLinesInRange(
	fr FileRange, omitProbablyCommon bool, maxCountInFile int) (num int) {
	maxCountInFile = MaxInt(1, maxCountInFile)
	for n := 0; n < fr.LineCount(); n++ {
		lp := fr.LinePosAtOffset(n)
		if omitProbablyCommon && lp.ProbablyCommon {
			continue
		}
		if int(lp.CountInFile) > maxCountInFile {
			continue
		}
		num++
	}
	return
}
