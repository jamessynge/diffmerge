package main

// Experimenting with how to measure the number of spaces a tab represents,
// and the number of spaces in typical indentations.

import (
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/davecgh/go-spew/spew"
	"github.com/jamessynge/diffmerge/dm"
	"github.com/jamessynge/go_io/fileio"
	"github.com/jamessynge/go_io/goioutil"
)

func FailWithMessage(showUsage bool, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	glog.Error(msg)
	fmt.Fprintln(os.Stderr, msg)
	if showUsage {
		flag.Usage()
	}
	os.Exit(1)
}

func ReadAFile(fileName string) *dm.File {
	file, err := dm.ReadFile(fileName)
	if err != nil {
		FailWithMessage(false, "Failed to read file %s: %s", fileName, err)
	}
	return file
}

var (
	knownExtensions = make(map[string]bool)
	textTypes = make(map[string]int)
	nonTextTypes = make(map[string]int)
	ignoreDirs = make(map[string]bool)
)

func init() {
	for _, ext := range strings.Split("cpp,c,cc,h,txt,go,java,js,html,xml,css", ",") {
		knownExtensions["." + ext] = true
	}
	for _, ext := range strings.Split("a,o,zip,png,jpg,jpeg,exe,doc,obj", ",") {
		knownExtensions["." + ext] = false
	}
	for _, name := range strings.Split(".settings,bin,war,pkg,classes", ",") {
		ignoreDirs[name] = true
	}
}

func fileExtImpliesText(ext string) (yes, unknown bool) {
	defer func() {
		glog.Infof("'%s' -> yes=%v   unknown=%v", ext, yes, unknown)
	}()

	if ext == "" {
		unknown = true
		return
	}
	if v, ok := knownExtensions[ext]; ok {
		yes = v
		return
	}
	// Take a guess based on past files.
	if nonTextTypes[ext] - textTypes[ext] > 5 {
		// Probably binary.
		glog.Infof("Past evidence indicates a binary extension: %s", ext)
		return
	}
	if textTypes[ext] - nonTextTypes[ext] > 5 {
		// Probably text.
		glog.Infof("Past evidence indicates a text extension: %s", ext)
		return
	}
	mt := mime.TypeByExtension(ext)
	if (strings.HasPrefix(mt, "text/") ||
			strings.HasSuffix(mt, "+xml") ||
			strings.HasSuffix(mt, ".json") ||
	    strings.HasSuffix(mt, "+json")) {
		// Most likely text.
		yes = true
		glog.Infof("Most likely a text extension: %s", ext)
		return
	}
	if (strings.HasPrefix(mt, "audio/") ||
			strings.HasPrefix(mt, "image/") ||
			strings.HasPrefix(mt, "video/")) {
		// Almost certainly not text.
		glog.Infof("Most likely a binary extension: %s", ext)
		return
	}
	unknown = true
	return
}

func parentDirectoryIsIgnored(input string) bool {
//	glog.Infof("parentDirectoryIsIgnored input=%s", input)
	vn := filepath.VolumeName(input)
	vp := input[len(vn):]
	parts := fileio.SplitPath(vp)
	for _, part := range parts {
//		glog.Infof("parentDirectoryIsIgnored part=%s", part)
		if ignoreDirs[part] {
//			glog.Infof("File in ignored directory %q: %s", part, input)
			return true
		}
	}
	return false
}

func fileContainsText(fileName string) (isText bool) {
	// Read the first hunk of the file, and determine if that is text.
	f, err := os.Open(fileName)
	if err != nil {
		return false
	}
	defer f.Close()
	var capacity int64 = 4096
	if fi, err := f.Stat(); err == nil {
		if size := fi.Size(); size < capacity {
			capacity = size
		}
	}
	buf := make([]byte, capacity)
	n, err := f.Read(buf)
	glog.Infof("Read %d bytes from: %s", n, fileName)
	if err != nil || n == 0 {
		return false
	}
	isGraphicAscii, isGraphicUtf8 := goioutil.IsGraphicAsciiOrUtf8(buf[0:n])
	return isGraphicAscii || isGraphicUtf8
}

func ReadArgDir(dir string, stats *dm.LeadingWhitespaceStatistics) {
	walkFn := func(fp string, info os.FileInfo, err error) error {
		glog.V(1).Infoln("walkFn", fp)
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if parentDirectoryIsIgnored(fp) {
			glog.V(1).Infof("File in ignored directory: %s", fp)
			return nil
		}
		ext := filepath.Ext(fp)
		yes, unknown := fileExtImpliesText(ext)
		if unknown {
			yes = fileContainsText(fp)
			if ext != "" {
				if yes {
					glog.Infof("File probably text: %s", fp)
					textTypes[ext]++
				} else {
					glog.Infof("File probably binary: %s", fp)
					nonTextTypes[ext]++
				}
			}
		}
		if !yes {
			return nil
		}
		file, err := dm.ReadFile(fp)
		if err != nil {
			FailWithMessage(false, "Failed to read file %s: %s", fp, err)
		}
		stats.AddFile(file)
		return nil
	}
	glog.Infoln("Searching under root", dir)
  err := filepath.Walk(dir, walkFn)
	if err != nil {
		FailWithMessage(false, "Failed searching directory %s: %s", dir, err)
	}
	return
}

func ReadArgFiles(arg string, stats *dm.LeadingWhitespaceStatistics) {
	matches, err := filepath.Glob(arg)
	if err != nil {
		FailWithMessage(false, "Unable to interpret %s: %s", arg, err)
	}
	for _, fileName := range matches {
		if fileio.IsDirectory(fileName) {
			ReadArgDir(fileName, stats)
		} else {
			file, err := dm.ReadFile(fileName)
			if err != nil {
				FailWithMessage(false, "Failed to read file %s: %s", fileName, err)
			}
			stats.AddFile(file)
		}
	}
	return
}

func main() {
	defer glog.Flush()
	diffConfig := &dm.DifferencerConfig{}
	diffConfig.CreateFlags(flag.CommandLine)
	flag.Parse() // Scan the arguments list

	cfg := spew.NewDefaultConfig()
	cfg.SortKeys = true
	cfg.Dump("Spew config:\n", cfg)

	cmd := filepath.Base(os.Args[0])
	glog.V(1).Infoln("cmd =", cmd)

	var stats dm.LeadingWhitespaceStatistics
	for i := 0; i < flag.NArg(); i++ {
		ReadArgFiles(flag.Arg(i), &stats)
	}
	fmt.Println("Non-text extensions:")
	cfg.Dump(nonTextTypes)
	fmt.Println("Text extensions:")
	cfg.Dump(textTypes)
	
	fmt.Println()
	fmt.Println()
	stats.ComputeFractions()

	fmt.Println("Whitespace info:")
	cfg.Dump(stats)
}
