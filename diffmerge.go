// Experiments with creating an alternate to diff3 and merge, motivated
// by my annoyance with these issues:
//
// 1) Ordered lists are not auto-merged. For example, C++ include lists
//    and build dependencies are typically in lexically sorted ordered,
//    so the merge tool should check to see if conflicting lines are
//    part of such a list in all 3 files, and if so treat them as
//		ordered sets, where
//    we don't care about the conflicting lines, but rather about the
//    set operations performed: insert and remove; and those operations
//    won't be in conflict.
//
// 2) Mis-alignment of matches for added functions. For example, we  desire
//    this, where are the lines of the new function have been marked new:
//
//      $diff -y file2 file1
//      void func1() {            void func1() {
//        x += 1                     x += 1
//      }                         }
//
//                              > void functhreehalves() {
//                              >   x += 1.5
//                              > }
//                              >
//      void func2() {            void func2() {
//        x += 2                    x += 2
//      }                         }
//
//    However, we often get this from diff, and hence from merge:
//
//      $diff -y file2 file1
//      void func1() {            void func1() {
//        x += 1                     x += 1
//      }                       > }
//                              >
//                              > void functhreehalves() {
//                              >   x += 1.5
//                                }
//
//      void func2() {            void func2() {
//        x += 2                    x += 2
//      }                         }
//
//    (example from http://fabiensanglard.net/git_code_review/diff.php).
//
// 3) Non-conflicting intra-line changes are not recognized as such; for example:
//
//      Base                 |   Yours                |   Theirs
//      if (y == x + 19) {   |   if (y == x + 17) {   |   if (z == x + 19) {
//                           |                ^^      |       ^
//
//    merge considers the lines in conflict:
//
//      <<<<<<< Yours
//      if (oldVar == x + 17) {
//      =======
//      if (newVar == x + 19) {
//      >>>>>>> Theirs
//
// 4) Moves of a block of lines should not be considered adds of new lines and
//    deletes of old lines, but rather recognized as a move so that changes
//    within the block of lines can be automatically merged.
//
// 5) If you've renamed a symbol in your file (i.e. changed ALL lines that
//    contain that symbol), and the other file adds uses of that symbol,
//    the tool should offer some means of automatically applying the renaming
//    to the new uses.  BONUS.
//
// 6) Changing the indentation of lines is treated as changing the entire line,
//    rather than being treated as separate from changing the characters to the
//    right of the indentation.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/jamessynge/diffmerge/dm"
)

var (
	pDiff3Flag = flag.Bool(
		"diff3", false, "Find difference between 3 files.")

	pStatusOnlyFlag = flag.Bool(
		"brief", false, "Report (via exit code) whether there were differences "+
			" (for diff) or conflicts (for diff3/merge) found.")

	pSideBySideFlag = flag.Bool(
		"side-by-side", true, "For diff of two files, display results side-by-side.")
)

// TODO Add support for merge(1)'s -L (label) flag, which can appear up to
// 3 times in the command line args. See https://play.golang.org/p/Ig5sm7jA14
// for an example of how to do this.

type CmdStatus int

const (
	ConflictFree CmdStatus = iota
	SomeConflicts
	AnError
	NoDifferences   = ConflictFree
	SomeDifferences = SomeConflicts
)

func FailWithMessage(showUsage bool, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	glog.Error(msg)
	fmt.Fprintln(os.Stderr, msg)
	if showUsage {
		flag.Usage()
	}
	os.Exit(int(AnError) & 0xff)
}

type cmdInputs struct {
	readingStdin bool
	fileNames    []string
	files        []*dm.File
	perm         os.FileMode

	// Output used for merge with 3 inputs and one output.
	outputFileName string

	diffConfig dm.DifferencerConfig
}

func (p *cmdInputs) AddInputFile(fileName string) {
	if fileName == "-" {
		if p.readingStdin {
			FailWithMessage(false, "Only one input file maybe '-' (for standard input).")
		}
		p.readingStdin = true
	}
	file, err := dm.ReadFile(fileName)
	if err != nil {
		FailWithMessage(false, "Failed to read file %s: %s", fileName, err)
		os.Exit(int(AnError) & 0xff)
	}
	p.fileNames = append(p.fileNames, fileName)
	p.files = append(p.files, file)
	if p.perm == 0 && fileName != "-" {
		fi, err := os.Stat(fileName)
		if err == nil {
			p.perm = fi.Mode().Perm()
		}
	}
}

func (p *cmdInputs) diff2Files(fromFile, toFile *dm.File) (pairs dm.BlockPairs, status CmdStatus) {
	pairs = dm.PerformDiff2(fromFile, toFile, p.diffConfig)
	glog.Flush()
	if len(pairs) == 1 && pairs[0].IsMatch {
		status = NoDifferences
	} else {
		status = SomeDifferences
	}
	return
}

func (p *cmdInputs) outputFile(f *dm.File) {
	if p.outputFileName != "" {
		err := ioutil.WriteFile(p.outputFileName, f.Body, p.perm)
		if err != nil {
			FailWithMessage(false, "Failed writing to %s; error: %s", p.outputFileName, err)
		}
	} else {
		r := bytes.NewReader(f.Body)
		_, err := io.Copy(os.Stdout, r)
		if err != nil {
			FailWithMessage(false, "Failed writing to stdout; error: %s", err)
		}
	}
}

func (p *cmdInputs) PerformDiff2() CmdStatus {
	fromFile, toFile := p.files[0], p.files[1]
	pairs, status := p.diff2Files(fromFile, toFile)
	if *pSideBySideFlag {
		dm.FormatSideBySide(fromFile, toFile, pairs, false, os.Stdout, dm.DefaultSideBySideConfig)
	} else {
		dm.FormatInterleaved(pairs, false, fromFile, toFile, os.Stdout, true)
	}
	return status
}

func (p *cmdInputs) PerformDiff3() CmdStatus {
	return AnError
}

func (p *cmdInputs) PerformMerge() CmdStatus {
	yours, base, theirs := p.files[0], p.files[1], p.files[2]
	b2yPairs, b2yStatus := p.diff2Files(base, yours)
	b2tPairs, b2tStatus := p.diff2Files(base, theirs)

	// len(.) == 1 expression included temporarily to supporess complaints about
	// unused vars.  TODO Remove.
	if b2yStatus == NoDifferences && len(b2yPairs) == 1 {
		// No changes in your file, so there can be no conflicts; output their file.
		p.outputFile(theirs)
		return ConflictFree
	} else if b2tStatus == NoDifferences && len(b2tPairs) == 1 {
		// No changes in their file, so there can be no conflicts; output your file.
		p.outputFile(yours)
		return ConflictFree
	}

	// Determine if there are possible conflicts. If so, then maybe do a diff of
	// yours vs. theirs, which may help eliminate the diff.
	// Start by sorting on the same index, base.
	dm.SortBlockPairsByAIndex(b2yPairs)
	dm.SortBlockPairsByAIndex(b2tPairs)

	return AnError // TODO Replace with correct value.
}

func main() {
	diffConfig := &dm.DifferencerConfig{}
	diffConfig.CreateFlags(flag.CommandLine)
	flag.Parse() // Scan the arguments list

	cmd := filepath.Base(os.Args[0])
	glog.V(1).Infoln("cmd =", cmd)

	nArgs := flag.NArg()
	if !(2 <= nArgs && nArgs <= 4) {
		FailWithMessage(true, "Wrong number of file arguments")
	}
	var ci cmdInputs
	ci.diffConfig = *diffConfig
	ci.AddInputFile(flag.Arg(0))
	ci.AddInputFile(flag.Arg(1))
	if nArgs > 2 {
		ci.AddInputFile(flag.Arg(2))
		if nArgs > 3 {
			ci.outputFileName = flag.Arg(3)
		}
	}

	var status CmdStatus
	if nArgs == 2 {
		status = ci.PerformDiff2()
	} else if *pDiff3Flag {
		status = ci.PerformDiff3()
	} else {
		status = ci.PerformMerge()
	}

	os.Exit(int(status) & 0xff)
}
