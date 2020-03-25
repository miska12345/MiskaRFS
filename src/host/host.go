// Package host export interface for host program
package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/miska12345/MiskaRFS/src/comm"
	"github.com/miska12345/MiskaRFS/src/fs"
	log "github.com/miska12345/MiskaRFS/src/logger"
	msg "github.com/miska12345/MiskaRFS/src/message"
	"github.com/miska12345/MiskaRFS/src/tcp2"
)

type Host struct {
	fs                 *fs.FSConfig
	Features           map[string]func(args ...string) *msg.Message
	Name               string
	Pass               string
	CurrentConnections int
	sync.Mutex
}

type client struct {
	Comm *comm.Comm
	Req  Request
}

type Request struct {
	Type string
	Body string
}

type ModuleConfig struct {
	Name           string
	BaseDir        string
	InvisibleFiles []string
	ReadOnly       bool
	AddFeatures    map[string]func(args ...string) *msg.Message
}

const ERR_REQUEST = -1

// Run starts the host on this machine with the given configuration
func Run(modConfig *ModuleConfig) (h *Host, err error) {
	h = new(Host)
	h.Name = modConfig.Name
	h.fs, err = fs.Init(modConfig.BaseDir, modConfig.InvisibleFiles, modConfig.ReadOnly)
	if err != nil {
		return
	}
	h.Features = make(map[string]func(args ...string) *msg.Message)

	err = h.initializeFileSystem()
	if err != nil {
		return
	}

	err = h.initializeCustomCMD(modConfig.AddFeatures)
	if err != nil {
		return
	}

	initializeCMD(h.Features)
	return h, h.start()
}

func (h *Host) start() (err error) {
	c, err := tcp2.ConnectToTCPServer("localhost:8080", "", h.Name)
	if err != nil {
		log.Error(err)
		return
	}
	for {
		data, err := c.Receive()
		if err != nil {
			log.Debug(err)
		}
		if bytes.Equal(data, []byte{1}) {
			continue
		}
		var req Request
		if err := json.Unmarshal(data, &req); err != nil {
			continue
		}
		go h.handleRequest(&client{
			Comm: c,
			Req:  req,
		})
	}
}

// AddFeature adds a command-func pair to the host for remote calls
func (h *Host) AddFeature(cmd string, f func(args ...string) *msg.Message) error {
	if _, ok := h.Features[cmd]; ok {
		return fmt.Errorf("CMD %s already exists", cmd)
	}
	h.Features[cmd] = f
	log.Debugf("Feature %s has been added", cmd)
	return nil
}

func (h *Host) handleCMD(cmd string) (res *msg.Message, err error) {
	fmt.Println(cmd)
	s := strings.Split(cmd, " ")
	fmt.Println(s)
	if len(s) > 0 {
		for i := 0; i < len(s); i++ {
			s[i] = strings.TrimSpace(s[i])
			fmt.Println(s[i])
		}
		if _, ok := h.Features[s[0]]; !ok {
			err = fmt.Errorf("No such command")
			return
		}
		res = h.Features[s[0]](s[1:]...)
	}
	return
}

func (h *Host) handleRequest(c *client) error {
	switch c.Req.Type {
	case "text/cmd":
		log.Debugf("Handle CMD %s", c.Req.Body)
		res, err := h.handleCMD(c.Req.Body)
		if err != nil {
			res = msg.New(msg.TYPE_ERROR, err.Error())
		}
		log.Debugf("Result: %s", res)
		bys, err := res.ConvertToNetForm()
		if err != nil {
			log.Error(err)
			return err
		}
		n, err := c.Comm.Write(bys)
		log.Debugf("Wrote %d bytes", n)
		return err
	default:
		log.Warnf("Unknown request type %s", c.Req.Type)
	}
	return nil
}

func initializeCMD(fs map[string]func(args ...string) *msg.Message) {
	fs["echo"] = func(args ...string) *msg.Message {
		if len(args) > 0 {
			return msg.New(msg.TYPE_RESPONSE, args[0])
		}
		return msg.New(msg.TYPE_ERROR, "<No Param>")
	}
}

func (h *Host) initializeFileSystem() (err error) {
	h.Features["ls"] = h.fs.ListFiles
	h.Features["cd"] = h.fs.CD
	h.Features["mkdir"] = h.fs.Mkdir
	h.Features["rm"] = h.fs.Remove
	return nil
}

func (h *Host) initializeCustomCMD(m map[string]func(args ...string) *msg.Message) error {
	for k := range m {
		if _, exist := h.Features[k]; exist {
			return fmt.Errorf("Duplicate function name found: %s", k)
		}
	}

	for k, v := range m {
		h.Features[k] = v
	}
	return nil
}
