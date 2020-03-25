package fs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	msg "github.com/miska12345/MiskaRFS/src/message"
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
		return nil, err
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

// ListFiles lists all the visible files under current directory
func (fs *FSConfig) ListFiles(args ...string) (fres *msg.Message) {
	var buf strings.Builder
	f, err := os.Open(fs.currentDir)
	if err != nil {
		return msg.New(msg.TYPE_ERROR, err.Error())
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return msg.New(msg.TYPE_ERROR, err.Error())
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
	return msg.New(msg.TYPE_RESPONSE, buf.String())
}

func (fs *FSConfig) CD(args ...string) *msg.Message {
	if len(args) == 0 {
		return msg.New(msg.TYPE_RESPONSE, fs.currentDir)
	}
	if _, err := os.Stat(fs.currentDir); os.IsNotExist(err) {
		return msg.New(msg.TYPE_ERROR, err.Error())
	}
	fs.currentDir = args[0]
	return msg.New(msg.TYPE_RESPONSE, fs.currentDir)
}

func (fs *FSConfig) Mkdir(args ...string) *msg.Message {
	for _, v := range args {
		err := os.Mkdir(filepath.Join(fs.currentDir, v), os.ModeDir)
		if err != nil {
			return msg.New(msg.TYPE_ERROR, err.Error())
		}
	}
	return fs.ListFiles()
}

func (fs *FSConfig) Remove(args ...string) *msg.Message {
	if fs.readOnly {
		return msg.New(msg.TYPE_ERROR, PERM_DENIED)
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
