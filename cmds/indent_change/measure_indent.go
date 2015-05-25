package main

// Experimenting with how to measure the number of spaces a tab
// represents, and the number of spaces in typical indentations.

import (
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"runtime"
		"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/davecgh/go-spew/spew"
	"github.com/jamessynge/diffmerge/dm"
	"github.com/jamessynge/go_io/fileio"
//	"github.com/jamessynge/go_io/glogext"
	"github.com/jamessynge/go_io/goioutil"
)

var (
	knownExtensions = make(map[string]bool)
	ignoreDirs = make(map[string]bool)
)
func init() {
	for _, ext := range strings.Split(
		"cpp,c,cc,h,txt,go,java,js,json,html,xml,css,yaml,py,awk", ",") {
		knownExtensions["." + ext] = true
	}
	for _, ext := range strings.Split(
		"a,o,zip,png,jpg,jpeg,exe,doc,obj,pdf,tar,rtf,gif,bz2,tgz,jar,class", ",") {
		knownExtensions["." + ext] = false
	}
	for _, name := range strings.Split(
		".settings,bin,war,pkg,classes,.git", ",") {
		ignoreDirs[name] = true
	}
}

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

func fileExtImpliesText(ext string) (yes, unknown bool) {
	defer func() {
		glog.V(2).Infof("'%s' -> yes=%v   unknown=%v", ext, yes, unknown)
	}()

	if ext == "" {
		unknown = true
		return
	}
	mt := mime.TypeByExtension(ext)
	if (strings.HasPrefix(mt, "text/") ||
			strings.HasSuffix(mt, "+xml") ||
			strings.HasSuffix(mt, ".json") ||
	    strings.HasSuffix(mt, "+json")) {
		// Most likely text.
		yes = true
		glog.V(1).Infof("Most likely a text extension: %s", ext)
		return
	}
	if (strings.HasPrefix(mt, "audio/") ||
			strings.HasPrefix(mt, "image/") ||
			strings.HasPrefix(mt, "video/")) {
		// Almost certainly not text.
		glog.V(1).Infof("Most likely a binary extension: %s", ext)
		return
	}
	unknown = true
	return
}

func parentDirectoryIsIgnored(input string, ignoreDirs map[string]bool) bool {
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

////////////////////////////////////////////////////////////////////////////////
// For fun, use go routines to better use multiple cpus.

type GoRtnCntrl struct {
	stopCh chan bool
	wg *sync.WaitGroup
}
func (p *GoRtnCntrl) ShouldStop() bool {
	glog.V(3).Info("ShouldStop?")
	select {
	case <- p.stopCh:
		glog.Info("ShouldStop = yes")
		return true
	default:
		glog.V(2).Info("ShouldStop = no")
		return false
	}
}

type FileTypeInfoResponse struct {
	fp string
	isText bool
}

type FileTypeInfoRequest struct {
	fp string
	ext string
	respCh chan *FileTypeInfoResponse
}

type FileTypeInfoUpdate struct {
	ext string
	isText bool
}

type FileTypeInfoManager struct {
	requestCh chan *FileTypeInfoRequest
	ctrl GoRtnCntrl
	// Filled in only once stopped.
	textTypes map[string]int
	nonTextTypes map[string]int
}

func MakeFileTypeInfoManager(ctrl GoRtnCntrl) *FileTypeInfoManager {
	p := &FileTypeInfoManager{
		requestCh: make(chan *FileTypeInfoRequest, runtime.NumCPU()),
		ctrl: ctrl,
	}

	stopCh := ctrl.stopCh

	sendResponse := func(req *FileTypeInfoRequest, isText bool, updateCh chan *FileTypeInfoUpdate) {
		resp := &FileTypeInfoResponse{
			fp: req.fp,
			isText: isText,
		}
		var update *FileTypeInfoUpdate
		if updateCh != nil {
			update = &FileTypeInfoUpdate{
				ext: req.ext,
				isText: isText,
			}
		}
		respCh := req.respCh
		for !(respCh == nil && updateCh == nil) {
			// Do whichever is first possible.
			select {
			case <- stopCh:
				return
			case respCh <- resp:
				respCh = nil
			case updateCh <- update:
				updateCh = nil
			}
		}
	}

	// Create some go routines that will handle actually reading files
	// to determine if they are text or not.
	checkFileContentCh := make(chan *FileTypeInfoRequest, runtime.NumCPU())
	updateCh := make(chan *FileTypeInfoUpdate, runtime.NumCPU())
	checkFileContentGR := func() {
		defer ctrl.wg.Done()
		for {
			select {
			case <- stopCh:
				return
			case req, ok := <- checkFileContentCh:
				if !ok {
					// No more requests.
					return
				}
				isText := fileContainsText(req.fp)
				sendResponse(req, isText, updateCh)
			}
		}
	}

	numGoRoutines := dm.MaxInt(1, runtime.NumCPU())
	for ; numGoRoutines > 0; numGoRoutines-- {
		ctrl.wg.Add(1)
		go checkFileContentGR()
	}

	// And create a go routine that will handle requests for file type.
	ctrl.wg.Add(1)
	go func() {
		defer ctrl.wg.Done()
		textTypes := make(map[string]int)
		nonTextTypes := make(map[string]int)
		defer func() {
			p.textTypes = textTypes
			p.nonTextTypes = nonTextTypes
		}()
		guesstimateType := func(ext string) (isText, isUnknown bool)  {
			if ext == "" {
				isUnknown = true
				return
			}
			if v, ok := knownExtensions[ext]; ok {
				isText = v
				return
			}
			// Take a guess based on past files.
			ntt, tt := nonTextTypes[ext], textTypes[ext]
			if (ntt == 0 && tt > 10) || (tt - ntt > 20) {
				// Probably text.
				glog.V(1).Infof("Past evidence (%d, %d) indicates a text extension: %s", ntt, tt, ext)
				isText = true
				return
			} else if (tt == 0 && ntt > 10) || (ntt - tt > 20) {
				// Probably binary.
				glog.V(1).Infof("Past evidence (%d, %d) indicates a binary extension: %s", ntt, tt, ext)
				return
			}
			isText, isUnknown = fileExtImpliesText(ext)
			return
		}
		for {
			select {
			case <- stopCh:
				return

			case update := <- updateCh:
				if update.isText {
					textTypes[update.ext]++
				} else {
					nonTextTypes[update.ext]++
				}

			case req, ok := <- p.requestCh:
				if !ok { return }
				isText, isUnknown := guesstimateType(req.ext)
				if isUnknown {
					// Ask another go routine to read the file in order to figure out
					// the answer. That routine will take care of providing the answer
					// to 
					select {
					case <- stopCh:
						return
					case checkFileContentCh <- req:
					}
				} else {
					sendResponse(req, isText, nil)
				}
			}
		}
	}()
	return p
}

func (p *FileTypeInfoManager) FindType(fp string) (isTextType bool) {
	if parentDirectoryIsIgnored(fp, ignoreDirs) {
		return false
	}
	ext := filepath.Ext(fp)
	if v, ok := knownExtensions[ext]; ok {
		return v
	}
	respCh := make(chan *FileTypeInfoResponse)
	req := &FileTypeInfoRequest{
		fp: fp,
		ext: ext,
		respCh: respCh,
	}

	// Make sure we aren't already stopped.
	if p.ctrl.ShouldStop() {
		return false
	}

	// Send the request, then wait for the response.
	select {
	case <- p.ctrl.stopCh:
		return false
	case p.requestCh <- req:
	}

	select {
	case <- p.ctrl.stopCh:
		return false
	case resp := <- respCh:
		return resp.isText
	}
}

////////////////////////////////////////////////////////////////////////////////
// Routine for finding candidate files given a dir.
func expandDir(dir string, candidateCh chan string, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	glog.V(1).Infoln("ENTER expandDir", dir)
	doStop := fmt.Errorf("")
	walkFn := func(fp string, info os.FileInfo, err error) error {
		glog.V(1).Infoln("walkFn", fp)
		if ctrl.ShouldStop() {
			return doStop
		}
		if err != nil {
			glog.Warning("Ignoring tree walk error: ", err)
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(fp)
			if ignoreDirs[base] {
				glog.Infof("Ignoring files in %s", fp)
				return filepath.SkipDir
			} else if fp == dir {
				return nil
			} else {
				ctrl.wg.Add(1)
				go expandDir(fp, candidateCh, ctrl)
				return filepath.SkipDir
			}
		}
		glog.V(1).Infof("Found candidate file %s", fp)
		select {
		case <- ctrl.stopCh:
			return doStop
		case candidateCh <- fp:
			return nil
		}
	}
	glog.V(1).Infoln("Searching under root", dir)
  err := filepath.Walk(dir, walkFn)
	if err != nil && err != doStop {
		FailWithMessage(false, "Failed searching directory %s: %s", dir, err)
	}
	glog.V(1).Infoln("EXIT expandDir", dir)
	return
}

// Routine for finding candidate files given an argument path glob.
func expandGlob(glob string, candidateCh chan string, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	glog.V(1).Infoln("ENTER expandGlob", glob)
	matches, err := filepath.Glob(glob)
	if err != nil {
		FailWithMessage(false, "Unable to interpret %s: %s", glob, err)
	}
	for _, fileName := range matches {
		if ctrl.ShouldStop() {
			return
		}
		if fileio.IsDirectory(fileName) {
			if len(matches) > 1 {
				fmt.Println(fmt.Sprintf("Expanding directory: %s", fileName))
			}
			ctrl.wg.Add(1)
			go expandDir(fileName, candidateCh, ctrl)
		} else {
			select {
			case <- ctrl.stopCh:
				return
			case candidateCh <- fileName:
			}
		}
	}
	glog.V(1).Infoln("EXIT expandGlob", glob)
}

// Find all the files identified by the command line args and send them to
// candidateCh, then close candidateCh.
func expandArgs(args []string, candidateCh chan string, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	glog.V(1).Infoln("ENTER expandArgs")

	nestedCtrl := ctrl
	nestedCtrl.wg = &sync.WaitGroup{}

	for _, arg := range args {
		nestedCtrl.wg.Add(1)
		go expandGlob(arg, candidateCh, nestedCtrl)
	}

	nestedCtrl.wg.Wait()
	close(candidateCh)
	glog.V(1).Infoln("EXIT expandArgs")
	fmt.Println(fmt.Sprintf("Done expanding %d args", len(args)))
}

// Forward those file paths received on candidateCh that are text files to textFileCh.
func filterCandidates(
	candidateCh chan string, textFileCh chan string,
	ftiMgr *FileTypeInfoManager, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	glog.V(1).Infoln("ENTER filterCandidates")
	defer glog.V(1).Infoln("EXIT filterCandidates")

	for {
		select {
		case <- ctrl.stopCh:
			return
		case fp, ok := <- candidateCh:
			if !ok {
				return
			}
			isText := ftiMgr.FindType(fp)
			if isText {
				select {
				case <- ctrl.stopCh:
					return
				case textFileCh <- fp:
				}
			}
		}
	}
}

func expandArgsToTextFiles(
	args []string, textFileCh chan string, numFilters int,
	ftiMgr *FileTypeInfoManager, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()

	nestedCtrl := ctrl
	nestedCtrl.wg = &sync.WaitGroup{}
	candidateCh := make(chan string, runtime.NumCPU())

	nestedCtrl.wg.Add(1)
	go expandArgs(args, candidateCh, nestedCtrl)

	numFilters = dm.MaxInt(1, numFilters)
	for i := 0; i < numFilters; i++ {
		nestedCtrl.wg.Add(1)
		go filterCandidates(candidateCh, textFileCh, ftiMgr, nestedCtrl)
	}

	nestedCtrl.wg.Wait()
	close(textFileCh)
}

func readFiles(textFileCh chan string, fileCh chan *dm.File, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	for {
		select {
		case <- ctrl.stopCh:
			return
		case fp, ok := <- textFileCh:
			if !ok {
				return
			}
			file := ReadAFile(fp)
			select {
			case <- ctrl.stopCh:
				return
			case fileCh <- file:
			}
		}
	}
}

func readAllFiles(numReaders int, textFileCh chan string, fileCh chan *dm.File, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	nestedCtrl := ctrl
	nestedCtrl.wg = &sync.WaitGroup{}
	numReaders = dm.MaxInt(1, numReaders)
	for i := 0; i < numReaders; i++ {
		nestedCtrl.wg.Add(1)
		go readFiles(textFileCh, fileCh, nestedCtrl)
	}
	nestedCtrl.wg.Wait()
	close(fileCh)
}

// Must be single threaded (w.r.t. stats access).
func collectStats(fileCh chan *dm.File, stats *dm.LeadingWhitespaceStatistics, ctrl GoRtnCntrl) {
	defer ctrl.wg.Done()
	for {
		select {
		case <- ctrl.stopCh:
			return
		case file, ok := <- fileCh:
			if !ok {
				return
			}
			stats.AddFile(file)
		}
	}
}

func main() {
	defer glog.Flush()
	diffConfig := &dm.DifferencerConfig{}
	diffConfig.CreateFlags(flag.CommandLine)
	flag.Parse() // Scan the arguments list

	goioutil.InitGOMAXPROCS()

	cfg := spew.NewDefaultConfig()
	cfg.SortKeys = true
	cfg.Dump("Spew config:\n", cfg)

	cmd := filepath.Base(os.Args[0])
	glog.V(1).Infoln("cmd =", cmd)

	ctrl := GoRtnCntrl{
		stopCh: make(chan bool),
		wg: &sync.WaitGroup{},
	}

	var stats dm.LeadingWhitespaceStatistics
	fileCh := make(chan *dm.File, runtime.NumCPU())

	ctrl.wg.Add(1)
	go collectStats(fileCh, &stats, ctrl)

	textFileCh := make(chan string, runtime.NumCPU())
	ctrl.wg.Add(1)
	go readAllFiles(runtime.NumCPU(), textFileCh, fileCh, ctrl)

	ftiCtrl := ctrl
	ftiCtrl.wg = &sync.WaitGroup{}
	ftiMgr := MakeFileTypeInfoManager(ftiCtrl)

	ctrl.wg.Add(1)
	expandArgsToTextFiles(flag.Args(), textFileCh, runtime.NumCPU(), ftiMgr, ctrl)

	// Wait for the main pipeline to complete.
	ctrl.wg.Wait()
	close(ctrl.stopCh)

	// Wait for the file type info manager to shutdown.
	ftiCtrl.wg.Wait()

	fmt.Println("Non-text extensions discovered:")
	cfg.Dump(ftiMgr.nonTextTypes)
	fmt.Println("Text extensions discovered:")
	cfg.Dump(ftiMgr.textTypes)

	fmt.Println()
	fmt.Println()
	stats.ComputeFractions()

	fmt.Println("Whitespace info:")
	cfg.Dump(stats)
}
