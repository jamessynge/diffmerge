package dm

import (
	"bytes"
	"hash/fnv"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
)

// TODO Come up with an indentation measure. For example, try to determine
// where tabs appear, and how many spaces they represent, then record that
// with each line.
// TODO Compute a second hash for each line, after normalizing (removing
// leading and trailing whitespace, etc.).

func normalizeLine(line []byte) []byte {
	line := bytes.TrimSpace(line)
	// TODO Maybe collapse multiple spaces inside line, maybe remove all
	// spaces, maybe normalize case.
	return line
}
/*
   527	func indexFunc(s []byte, f func(r rune) bool, truth bool) int {
   528		start := 0
   529		for start < len(s) {
   530			wid := 1
   531			r := rune(s[start])
   532			if r >= utf8.RuneSelf {
   533				r, wid = utf8.DecodeRune(s[start:])
   534			}
   535			if f(r) == truth {
   536				return start
   537			}
   538			start += wid
   539		}
   540		return -1
   541	}
   542	
   543	// lastIndexFunc is the same as LastIndexFunc except that if
   544	// truth==false, the sense of the predicate function is
   545	// inverted.
   546	func lastIndexFunc(s []byte, f func(r rune) bool, truth bool) int {
   547		for i := len(s); i > 0; {
   548			r, size := rune(s[i-1]), 1
   549			if r >= utf8.RuneSelf {
   550				r, size = utf8.DecodeLastRune(s[0:i])
   551			}
   552			i -= size
   553			if f(r) == truth {
   554				return i
   555			}
   556		}
   557		return -1
   558	}
   559	
*/




func ReadFile(name string) (*File, error) {
	body, err := ioutil.ReadFile(name)
	if err != nil {
		glog.Infof("Failed to read file %s: %s", name, err)
		return nil, err
	}
	glog.Infof("Loaded %d bytes from file %s", len(body), name)
	p := &File{
		Name:   name,
		Body:   body,
		Counts: make(map[uint32]int),
	}
	hasher := fnv.New32a()
	buf := bytes.NewBuffer(body)
	var pos int = 0
	for buf.Len() > 0 {
		line, err := buf.ReadBytes('\n')
		if line != nil {
			hasher.Reset()
			if _, err := hasher.Write(line); err != nil {
				return nil, err
			}
			index := len(p.Lines)
			hash := hasher.Sum32()
			length := len(line)

			normalizedLine, normalizedHash := normalizeLine(line), 0
			if len(normalizedLine) > 0 {
				hasher.Reset()
				if _, err := hasher.Write(normalizeLine(line)); err != nil {
					return nil, err
				}
				normalizedHash = hasher.Sum32()
			}
			
			p.Lines = append(p.Lines, LinePos{
				Start:  pos,
				Length: length,
				Index:  index,
				Hash:   hash,
				NormalizedHash: normalizedHash,
			})
			p.Counts[hash]++
			pos += length
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	return p, nil
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

func (p *File) UniqueLines() []LinePos {
	var result []LinePos
	for n := range p.Lines {
		if p.Counts[p.Lines[n].Hash] == 1 {
			result = append(result, p.Lines[n])
		}
	}
	return result
}

func (p *File) GetLineBytes(n int) []byte {
	if 0 <= n && n < len(p.Lines) {
		//		glog.Infof("GetLineBytes(%d) found LinePos: %v", n, p.Lines[n])
		s := p.Lines[n].Start
		l := p.Lines[n].Length
		return p.Body[s : s+l]
	}
	return nil
}

func (p *File) GetLineCount() int {
	return len(p.Lines)
}
