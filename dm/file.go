package dm

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path"

	"github.com/golang/glog"
)

// TODO Come up with an indentation measure. For example, try to determine
// where tabs appear, and how many spaces they represent, then record that
// with each line.

type File struct {
	Name  string    // Command line arg
	Body  []byte    // Body of the file
	Lines []LinePos // Locations and hashes of the file lines.

	FullRange  FileRange
	FileRanges map[IndexPair]FileRange
}

func (p *File) BriefDebugString() string {
	return fmt.Sprintf("File{\"%s\", %d lines, %d bytes}",
		path.Base(p.Name), len(p.Lines), len(p.Body))
}

func (p *File) GetFullRange() FileRange {
	return p.FullRange
}

func (p *File) MakeSubRange(start, length int) FileRange {
	glog.V(1).Infof("File.MakeSubRange [%d, %d)", start, start+length)
	lc := p.LineCount()
	if !(0 <= start && start+length <= lc && length >= 0) {
		glog.Fatalf("New range [%d, %d) is invalid (max length %d)",
			start, start+length, lc)
	}
	key := IndexPair{start, length}
	if p.FileRanges == nil {
		p.FileRanges = make(map[IndexPair]FileRange)
	} else if fr, ok := p.FileRanges[key]; ok {
		glog.V(1).Infof("File.MakeSubRange reusing existing FileRange")
		return fr
	}
	fr := &fileRange{
		file:   p,
		start:  start,
		length: length,
		beyond: start + length,
	}
	p.FileRanges[key] = fr
	return fr
}

func (p *File) Select(fn func(lp LinePos) bool) []LinePos {
	var result []LinePos
	for n := range p.Lines {
		if fn(p.Lines[n]) {
			result = append(result, p.Lines[n])
		}
	}
	return result
}

//func (p *File) UniqueLines() []LinePos {
//	var result []LinePos
//	for n := range p.Lines {
//		if p.Counts[p.Lines[n].Hash] == 1 {
//			result = append(result, p.Lines[n])
//		}
//	}
//	return result
//}

func (p *File) GetLineBytes(n int) []byte {
	if 0 <= n && n < len(p.Lines) {
		//		glog.Infof("GetLineBytes(%d) found LinePos: %v", n, p.Lines[n])
		s := p.Lines[n].Start
		l := p.Lines[n].Length
		return p.Body[s : s+l]
	}
	return nil
}

func (p *File) GetUnindentedLineBytes(n int) []byte {
	line := p.GetLineBytes(n)
	return removeIndent(line)
}

func (p *File) GetHashOfLine(n int) uint32 {
	return p.Lines[n].Hash
}

func (p *File) GetNormalizedHashOfLine(n int) uint32 {
	return p.Lines[n].NormalizedHash
}

func (p *File) LineCount() int {
	return len(p.Lines)
}

func countLeadingWhitespace(lineBytes []byte) (tabCount, spaceCount uint8) {
	length := len(lineBytes)
	// There are around 20 space characters in Unicode, but we're only handling
	// ASCII tab and space characters here.
	n := 0
	for ; n < length; n++ {
		if lineBytes[n] != '\t' {
			break
		}
	}
	numTabs := n
	for ; n < length; n++ {
		if lineBytes[n] != ' ' {
			break
		}
	}
	numSpaces := n - numTabs
	// Content starts after leading whitespace, any mixture of spaces and tabs.
	// If after the leading tabs, then leading spaces there is a tab, then we
	// declare that the line doesn't start with "well-formed" indentation, so
	// we can't as readily compare indentations. So, mark this line such that
	// we can detect this.
	if n < length && lineBytes[n] == '\t' {
		numTabs = 255
		numSpaces = 255
	}
	return uint8(MinInt(numTabs, 255)), uint8(MinInt(numSpaces, 255))
}

func ReadFile(name string) (*File, error) {
	body, err := ioutil.ReadFile(name)
	if err != nil {
		glog.Infof("Failed to read file %s: %s", name, err)
		return nil, err
	}
	glog.Infof("Loaded %d bytes from file %s", len(body), name)
	p := &File{
		Name: name,
		Body: body,
		//		Counts: make(map[uint32]int),
	}
	hasher := theLineHasher
	buf := bytes.NewBuffer(body)
	var pos int = 0
	for buf.Len() > 0 {
		line, err := buf.ReadBytes('\n')
		if line != nil {
			index := len(p.Lines)
			length := len(line)
			tabCount, spaceCount := countLeadingWhitespace(line)
			unindentedLine := removeIndent(line)
			hash, normalizedHash := hasher.Compute2(line, unindentedLine)
			normalizedLine := normalizeLine(unindentedLine)
			normalizedLength := len(normalizedLine)
			probablyCommon := normalizedLength == 0 || ComputeIsProbablyCommon(normalizedLine)
			p.Lines = append(p.Lines, LinePos{
				Start:            pos,
				Length:           length,
				Index:            index,
				Hash:             hash,
				NormalizedHash:   normalizedHash,
				NormalizedLength: uint8(MinInt(normalizedLength, 255)),
				ProbablyCommon:   probablyCommon,
				LeadingTabs:      tabCount,
				LeadingSpaces:    spaceCount,
			})
			pos += length
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	// Compute LinePos.CountInFile values.
	counts := make(map[uint32]int)
	for n := range p.Lines {
		counts[p.Lines[n].NormalizedHash]++
	}
	for n := range p.Lines {
		lp := &p.Lines[n]
		count := counts[lp.NormalizedHash]
		if count > 255 {
			lp.CountInFile = 255
		} else {
			lp.CountInFile = uint8(count)
		}
	}

	p.FullRange = p.MakeSubRange(0, p.LineCount())
	return p, nil
}
