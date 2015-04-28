package dm

import ()

// TODO Implement code for detecting indentation changes. Specifically:
// 1) Given a BlockPair.NormalizedMatch with several lines, determine if all
//    lines have the same change to their indentation (probably won't work
//    well for files that use both spaces and tabs for indentation). The change
//    might be the addition or removal of the same leading string of whitespace (could
//    also check for a change from tab indentation to space indentation, or
//    vice versa).
// 2) Given such a change, see if the adjacent (in AIndex & BIndex terms)
//    non-match lines have the same leading string; if so extend the range of
//    the detected indentation change, and continue to ask the same question
//    of the next or previous NormalizedMatch; lather, rinse, repeat until
//    we have a contiguous sequence of lines with the same change to their
//    indentation (could also look for a minimum indentation change, with
//    the possibility that there might be other indentation changes nested
//    inside the first).
//
//
// Alternate/supporting idea: add a new field, LinePos.IndentationDepth
// which estimates the amount of indentation the line starts with (I say
// estimates because some files may used mixed spaces and tabs for indentation,
// and then we need a way to know the number of spaces that a tab represents).
// For now, could use a command-line arg to specify that, but could also run
// through the file and count the number of spaces that appear in each row after
// a tab, or BOL, and use the population stats to estimate the number of spaces
// that a tab represents (i.e. if that number is 4, then 0, 1, 2, and 3 will
// be fairly common, but there should be a sharp drop in the frequency of 4
// spaces).

type LeadingWhitespaceStatistics struct {
	NumLeadingTabs map[uint8]int
	NumLeadingSpaces map[uint8]int
	NumLeadingSpacesAfterTab map[uint8]int
}

func MeasureLeadingWhitespace(files ...*File) (stats LeadingWhitespaceStatistics) {
	stats.NumLeadingTabs = make(map[uint8]int)
	stats.NumLeadingSpaces = make(map[uint8]int)
	stats.NumLeadingSpacesAfterTab = make(map[uint8]int)

	for _, file := range files {
		lines := file.Lines
		for n := range lines {
			lp := &lines[n]
			lp.


	return
}










//func GuessTabSpaces(