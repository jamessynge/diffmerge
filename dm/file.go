package dm

import (
	"bytes"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
)

func ReadFile(name string) (*File, error) {
	body, err := ioutil.ReadFile(name)
	if err != nil {
		log.Printf("Failed to read file %s: %s", name, err)
		return nil, err
	}
	log.Printf("Loaded %d bytes from file %s", len(body), name)
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
			p.Lines = append(p.Lines, LinePos{
				Start:  pos,
				Length: len(line),
				Index:  index,
				Hash:   hash,
			})
			p.Counts[hash]++
			pos += len(line)
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
