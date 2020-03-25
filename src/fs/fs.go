package fs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	log "github.com/miska12345/MiskaRFS/src/logger"
)

const PERM_DENIED = "PERMISSION DENIED"

type FSConfig struct {
	baseDir        string
	invisibleFiles map[string]bool
	readOnly       bool
	lastAccessed   time.Time
	currentDir     string
}

func Init(baseDir string, invisibleFiles []string, readOnly bool) (fsc *FSConfig, err error) {
	fsc = new(FSConfig)
	fsc.baseDir = baseDir

	_, err = ioutil.ReadDir(fsc.baseDir)
	if err != nil {
		return
	}

	fsc.invisibleFiles = make(map[string]bool)
	for _, v := range invisibleFiles {
		fsc.invisibleFiles[v] = true
	}

	fsc.readOnly = readOnly
	fsc.currentDir = baseDir
	fsc.lastAccessed = time.Now()
	return
}

func (fs *FSConfig) ListFiles(args ...string) (res string) {
	var buf strings.Builder
	f, err := os.Open(fs.currentDir)
	if err != nil {
		return err.Error()
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return err.Error()
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsDir() && !list[j].IsDir() {
			return true
		} else if !list[i].IsDir() && list[j].IsDir() {
			return false
		}
		return list[i].Name() < list[j].Name()
	})
	buf.WriteString(fmt.Sprintf("\n\tDirectory: %s\n\n", fs.currentDir))
	for _, v := range list {
		fname := v.Name()

		// File filter
		if _, invisible := fs.invisibleFiles[fname]; invisible {
			continue
		}
		if v.IsDir() {
			fname = "." + fname
		}
		buf.WriteString(fmt.Sprintf("%d/%d/%d\t%s\n", v.ModTime().Month(), v.ModTime().Day(), v.ModTime().Year(), fname))
	}
	return buf.String()
}

func (fs *FSConfig) CD(args ...string) string {
	if len(args) == 0 {
		return fs.currentDir
	}
	if _, err := os.Stat(fs.currentDir); os.IsNotExist(err) {

	}
	fs.currentDir = args[0]
	return fs.currentDir
}

func (fs *FSConfig) Mkdir(args ...string) string {
	for _, v := range args {
		err := os.Mkdir(filepath.Join(fs.currentDir, v), os.ModeDir)
		if err != nil {
			log.Error(err)
		}
	}
	return fs.ListFiles()
}

func (fs *FSConfig) Remove(args ...string) string {
	if fs.readOnly {
		return PERM_DENIED
	}
	for _, v := range args {
		// Skip all the invisible files
		if _, inv := fs.invisibleFiles[v]; inv {
			continue
		}
		os.Remove(filepath.Join(fs.currentDir, v))
	}
	return fs.ListFiles()
}
