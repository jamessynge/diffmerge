package dm

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
)

// TODO Come up with an indentation measure. For example, try to determine
// where tabs appear, and how many spaces they represent, then record that
// with each line.

type File struct {
	Name  string    // Command line arg
	Body  []byte    // Body of the file
	Lines []LinePos // Locations and hashes of the file lines.

	fullRange FileRange
}

func (p *File) GetFullRange() FileRange {
	return p.fullRange
}

func (p *File) GetSubRange(start, length int) FileRange {
	return CreateFileRange(p, start, length)
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

func (p *File) GetHashOfLine(n int) uint32 {
	return p.Lines[n].Hash
}

func (p *File) GetNormalizedHashOfLine(n int) uint32 {
	return p.Lines[n].NormalizedHash
}

func (p *File) GetLineCount() int {
	return len(p.Lines)
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
			unindentedLine := removeIndent(line)
			hash, normalizedHash := hasher.Compute2(line, unindentedLine)
			probablyCommon := len(unindentedLine) == 0
			if !probablyCommon {
				normalizedLine := normalizeLine(unindentedLine)
				probablyCommon = computeIsProbablyCommon(normalizedLine)
			}
			p.Lines = append(p.Lines, LinePos{
				Start:          pos,
				Length:         length,
				Index:          index,
				Hash:           hash,
				NormalizedHash: normalizedHash,
				ProbablyCommon: probablyCommon,
			})
			//			p.Counts[hash]++
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

	p.fullRange = CreateFileRange(p, 0, p.GetLineCount())
	return p, nil
}
