package unused

import ()

// Want a unique id for each unique string. Not sure if it is better to store
// them (as in here in UniqueStrings), or to just hash them each time they
// are encountered (after all, that is probably just what go is doing under
// the covers here).
type UniqueStrings struct {
	m map[string]int
	v []string
}

func NewUniqueStrings() *UniqueStrings {
	p := &UniqueStrings{}
	p.m = make(map[string]int)
	return p
}
func (p *UniqueStrings) Intern(s string) int {
	if n, ok := p.m[s]; ok {
		return n
	}
	// Gauranteeing that the lowest value will be 1, not 0, so that we can tell
	// an uninitialized field from the first entry.
	p.v = append(p.v, s)
	var n = len(p.v)
	p.m[s] = n
	return n
}
func (p *UniqueStrings) Get(n int) string {
	return p.v[n-1]
}
