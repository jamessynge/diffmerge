package dm

import ()

// Aim here is to be able to represent two files, or two ranges, one in each
// of two files, as a single object, and to be able to represent knowledge
// about those ranges (e.g. a match, an indentation change, a move), and
// their relationships to other FRP's (e.g. the root object might contain
// many other FRP's, directly and indirectly). This interface might expose
// operations that modify itself (useful for maintaining relationships, because
// object identity) and/or create new objects (perhaps in a functional fashion,
// leaving the original object unmodified).

type FileRangePair interface {
	A() FileRange
	B() FileRange

	AddChild(child FileRangePair)
	AddChildren(children ...FileRangePair)

	HasChildren() bool
	GetChildren() []FileRangePair
	NumChildren()

	VisitChildren(byAIndex bool, visitor func(child FileRangePair) bool)
}
