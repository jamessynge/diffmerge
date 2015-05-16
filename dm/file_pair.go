package dm

import (
	"fmt"
	"github.com/golang/glog"
)

// Aim here is to be able to represent two files being compared as a single object.

type FilePair interface {
	AFile() *File
	BFile() *File

	FullFileRangePair() FileRangePair
	MakeSubRangePair(aIndex, aLength, bIndex, bLength int) FileRangePair

	MakeFileRangePair(aRange, bRange FileRange) FileRangePair

	ALength() int
	BLength() int

	BriefDebugString() string

	CompareFileLines(aIndex, bIndex int, maxRareOccurrences uint8) (equal, approx, rare bool)

	CanFillGapWithMatches(pair1, pair2 *BlockPair) (equal, approx bool)
}

type FourIndices [4]int

type filePair struct {
	aFile, bFile *File

	fileRangePairs map[FourIndices]FileRangePair
}

func MakeFilePair(aFile, bFile *File) FilePair {
	return &filePair{
		aFile:          aFile,
		bFile:          bFile,
		fileRangePairs: make(map[FourIndices]FileRangePair),
	}
}

func (p *filePair) FullFileRangePair() FileRangePair {
	return p.MakeSubRangePair(0, p.ALength(), 0, p.BLength())
}

func (p *filePair) MakeSubRangePair(aIndex, aLength, bIndex, bLength int) FileRangePair {
	glog.V(1).Infof("filePair.MakeSubRangePair AIndices: [%d, +%d),  BIndices: [%d, +%d)",
		aIndex, aLength, bIndex, bLength)
	key := FourIndices{aIndex, aLength, bIndex, bLength}
	if frp, ok := p.fileRangePairs[key]; ok {
		return frp
	}
	aRange := p.aFile.MakeSubRange(aIndex, aLength)
	bRange := p.bFile.MakeSubRange(bIndex, bLength)
	frp := &frpImpl{
		filePair: p,
		aRange:   aRange,
		bRange:   bRange,
		aLength:  aLength,
		bLength:  bLength,
	}
	return frp
}

func (p *filePair) MakeFileRangePair(aRange, bRange FileRange) FileRangePair {
	glog.V(1).Infof("filePair.MakeFileRangePair aRange: %s   bRange: %s", aRange, bRange)
	if p.aFile != aRange.File() {
		glog.Fatalf("aRange is for the wrong file!\nExpected: %s\n  Actual: %s",
			p.aFile.BriefDebugString(),
			aRange.File().BriefDebugString())
	}
	if p.bFile != bRange.File() {
		glog.Fatalf("bRange is for the wrong file!\nExpected: %s\n  Actual: %s",
			p.bFile.BriefDebugString(),
			bRange.File().BriefDebugString())
	}
	aIndex, aLength := aRange.FirstIndex(), aRange.Length()
	bIndex, bLength := bRange.FirstIndex(), bRange.Length()
	key := FourIndices{aIndex, aLength, bIndex, bLength}
	if frp, ok := p.fileRangePairs[key]; ok {
		return frp
	}
	frp := &frpImpl{
		filePair: p,
		aRange:   aRange,
		bRange:   bRange,
		aLength:  aLength,
		bLength:  bLength,
	}
	return frp
}

func (p *filePair) AFile() *File {
	return p.aFile
}

func (p *filePair) BFile() *File {
	return p.bFile
}

func (p *filePair) ALength() int { return p.aFile.LineCount() }

func (p *filePair) BLength() int { return p.bFile.LineCount() }

func (p *filePair) BriefDebugString() string {
	return fmt.Sprintf("FilePair{A: %s,  B: %s}",
		p.aFile.BriefDebugString(), p.bFile.BriefDebugString())
}

// Not comparing actual content, just hashes and lengths.
func (p *filePair) CompareFileLines(
	aIndex, bIndex int, maxRareOccurrences uint8) (equal, approx, rare bool) {
	aLP := &p.aFile.Lines[aIndex]
	bLP := &p.bFile.Lines[bIndex]
	if glog.V(2) {
		glog.Infof("CompareFileLines(%d, %d, %d) aLP: %v    bLP: %v",
			aIndex, bIndex, maxRareOccurrences, *aLP, *bLP)
		defer func() {
			glog.Infof("CompareFileLines(%d, %d, %d) -> %v, %v, %v\nA: %q\nB: %q",
				aIndex, bIndex, maxRareOccurrences, equal, approx, rare,
				p.aFile.GetUnindentedLineBytes(aIndex), p.bFile.GetUnindentedLineBytes(bIndex))
		}()
	}
	if aLP.NormalizedHash != bLP.NormalizedHash {
		return
	}
	if aLP.NormalizedLength != bLP.NormalizedLength {
		return
	}
	approx = true
	if aLP.Hash == bLP.Hash && aLP.Length == bLP.Length {
		equal = true
	}
	if aLP.ProbablyCommon || bLP.ProbablyCommon {
		return
	}
	if maxRareOccurrences < aLP.CountInFile {
		return
	}
	if maxRareOccurrences < bLP.CountInFile {
		return
	}
	rare = true
	return
}

func (p *filePair) CanFillGapWithMatches(pair1, pair2 *BlockPair) (exactlyEqual, onlyApprox bool) {
	aLo, bLo := pair1.ABeyond(), pair1.BBeyond()
	aHi, bHi := pair2.AIndex, pair2.BIndex
	aLength := aHi - aLo
	bLength := bHi - bLo
	if aLength <= 0 || aLength != bLength {
		return
	}
	allExact := true
	for aLo < aHi && bLo < bHi {
		lineEqual, lineApprox, _ := p.CompareFileLines(aLo, bLo, 0)
		if !lineApprox {
			return
		}
		allExact = allExact && lineEqual
		aLo++
		bLo++
	}
	return allExact, !allExact
}
