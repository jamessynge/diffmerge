package dm

import (
	"crypto/rand"
	"hash"
	"hash/fnv"
	"io"

	"github.com/golang/glog"
)

type LineHasher interface {
	Compute(line []byte) (fullHash, normalizedHash uint32)
	Compute2(line, normalizedLine []byte) (fullHash, normalizedHash uint32)
}

type HashPositions map[uint32][]int

func (m HashPositions) CopyMap() HashPositions {
	result := make(HashPositions)
	for k, v := range m {
		result[k] = append([]int(nil), v...)
	}
	return result
}

// Global to be used in most situations. During testing may replace between
// tests.
var theLineHasher = createLineHasher()

// Function that writes the current hash seed into the supplied hasher.
// Replaceable for testing (i.e. can just reassign it to the output of another
// call to createWriteHashSeed()).
var writeHashSeedFn = createWriteHashSeed()

// Encapsulate the seed, so it isn't easy to muck up, as it needs to be the
// same for all files involved in a diff.
func createWriteHashSeed() func(w io.Writer) {
	seed := make([]byte, 4)
	if n, err := rand.Read(seed); err != nil || n != len(seed) {
		glog.Fatalf("Unable to generate seed for hasher!  n=%d, err=%v", n, err)
	}
	return func(w io.Writer) {
		if _, err := w.Write(seed); err != nil {
			glog.Fatalf("Unable to write seed to Hash32! err=%v", err)
		}
	}
}

type hash32WithSeed struct {
	h hash.Hash32
}

func (p *hash32WithSeed) Hash(line []byte) uint32 {
	p.h.Reset()
	writeHashSeedFn(p.h)
	if _, err := p.h.Write(line); err != nil {
		glog.Fatalf("Unable to write to Hash32! err=%v", err)
	}
	return p.h.Sum32()
}

type lineHasher struct {
	hf, hn hash32WithSeed
}

func createLineHasher() LineHasher {
	return &lineHasher{
		hf: hash32WithSeed{fnv.New32()},
		hn: hash32WithSeed{fnv.New32a()},
	}
}

func (p *lineHasher) Compute(line []byte) (fullHash, normalizedHash uint32) {
	if len(line) > 0 {
		fullHash = p.hf.Hash(line)
		line = normalizeLine(line)
		if len(line) > 0 {
			normalizedHash = p.hn.Hash(line)
		}
	}
	return
}
func (p *lineHasher) Compute2(line, normalizedLine []byte) (fullHash, normalizedHash uint32) {
	if len(line) > 0 {
		fullHash = p.hf.Hash(line)
	}
	if len(normalizedLine) > 0 {
		normalizedHash = p.hn.Hash(line)
	}
	return
}
