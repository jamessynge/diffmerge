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
	"flag"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/jamessynge/diffmerge/dm"
)

var (
	pDiff3Flag = flag.Bool(
		"diff3", false, "Find difference between 3 files.")
)

func ReadFileOrDie(name string) *dm.File {
	// TODO Support 1 file named "-", which means read from stdin.
	f, err := dm.ReadFile(name)
	if err != nil {
		glog.Fatalf("Failed to read file %s: %s", name, err)
	}
	glog.Infof("Loaded %d lines from %s", len(f.Lines), f.Name)
	return f
}

func ComputeBlockPairs(fromFile, toFile *dm.File) []*dm.BlockPair {
	bms := dm.BramCohensPatienceDiff(fromFile, toFile)

	glog.Info("Initial matches found:")
	for n, bm := range bms {
		glog.Infof("%d: %v", n, bm)
	}
	glog.Infoln()

	pairs := dm.BlockMatchesToBlockPairs(fromFile, toFile, bms)

	glog.Info("BlockPairs:")
	for n, pair := range pairs {
		glog.Infof("%d: %v", n, pair)
	}
	glog.Infoln()

	return pairs
}

type CmdStatus int

const (
	ConflictFree CmdStatus = iota
	SomeConflicts
	AnError
)

func Diff2Files(fromFile, toFile *dm.File) CmdStatus {
	pairs := ComputeBlockPairs(fromFile, toFile)

	dm.FormatInterleaved(pairs, true, fromFile, toFile, os.Stdout, true)

	if len(pairs) == 1 && pairs[0].IsMatch {
		return ConflictFree
	} else {
		return SomeConflicts
	}
}
func Diff3Files(yours, origin, theirs *dm.File) CmdStatus {
	return AnError
}
func Merge3Files(yours, origin, theirs *dm.File) CmdStatus {
	return AnError
}

func main() {
	diffConfig := &dm.DifferencerConfig{}
	diffConfig.CreateFlags(flag.CommandLine)
	flag.Parse() // Scan the arguments list

	cmd := filepath.Base(os.Args[0])
	glog.V(1).Infoln("cmd =", cmd)

	var status CmdStatus
	switch flag.NArg() {
	case 2:
		fromFile := ReadFileOrDie(flag.Arg(0))
		toFile := ReadFileOrDie(flag.Arg(1))
		pairs := dm.PerformDiff(fromFile, toFile, *diffConfig)
		
		dm.FormatInterleaved(pairs, true, fromFile, toFile, os.Stdout, true)

		if len(pairs) == 1 && pairs[0].IsMatch {
			status = ConflictFree
		} else {
			status = SomeConflicts
		}
		
//		status = Diff2Files(fromFile, toFile)

	case 3:
		yours := ReadFileOrDie(flag.Arg(0))
		origin := ReadFileOrDie(flag.Arg(1))
		theirs := ReadFileOrDie(flag.Arg(2))
		if *pDiff3Flag {
			status = Diff3Files(yours, origin, theirs)
		} else {
			status = Merge3Files(yours, origin, theirs)
		}

	default:
		glog.Errorf("Command requires 2 or 3 arguments, not %d", flag.NArg())
		flag.Usage()
		status = AnError
	}

	os.Exit(int(status) & 0xff)
}
