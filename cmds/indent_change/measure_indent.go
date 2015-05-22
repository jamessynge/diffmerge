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

func FailWithMessage(showUsage bool, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	glog.Error(msg)
	fmt.Fprintln(os.Stderr, msg)
	if showUsage {
		flag.Usage()
	}
	os.Exit(int(AnError) & 0xff)
}

func main() {
	diffConfig := &dm.DifferencerConfig{}
	diffConfig.CreateFlags(flag.CommandLine)
	flag.Parse() // Scan the arguments list

	cmd := filepath.Base(os.Args[0])
	glog.V(1).Infoln("cmd =", cmd)

	var files []*dm.File
	for i := 0; i < flag.NArg(); i++ {
		fileName := flag.Arg(i)
		file, err := dm.ReadFile(fileName)
		if err != nil {
			FailWithMessage(false, "Failed to read file %s: %s", fileName, err)
			os.Exit(int(AnError) & 0xff)
		}
	
	
	}



	file, err := dm.ReadFile(fileName)
	if err != nil {
		FailWithMessage(false, "Failed to read file %s: %s", fileName, err)
		os.Exit(int(AnError) & 0xff)
	}


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
