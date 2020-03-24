package tcp

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/schollz/croc/v8/src/comm"
	"github.com/schollz/croc/v8/src/crypt"
	"github.com/schollz/croc/v8/src/models"
	log "github.com/schollz/logger"
	"github.com/schollz/pake/v2"
)

type server struct {
	port       string
	debugLevel string
	banner     string
	password   string
	rooms      roomMap
}

type roomInfo struct {
	first    *comm.Comm
	second   *comm.Comm
	password string
	opened   time.Time
	full     bool
}

type roomMap struct {
	rooms map[string]roomInfo
	sync.Mutex
}

var timeToRoomDeletion = 10 * time.Minute

// Run starts a tcp listener, run async
func Run(debugLevel, port, password string, banner ...string) (err error) {
	s := new(server)
	s.port = port
	s.password = password
	s.debugLevel = debugLevel
	if len(banner) > 0 {
		s.banner = banner[0]
	}
	return s.start()
}

func (s *server) start() (err error) {
	log.SetLevel(s.debugLevel)
	log.Debugf("starting with password '%s'", s.password)
	s.rooms.Lock()
	s.rooms.rooms = make(map[string]roomInfo)
	s.rooms.Unlock()

	// delete old rooms
	go func() {
		for {
			time.Sleep(timeToRoomDeletion)
			roomsToDelete := []string{}
			s.rooms.Lock()
			for room := range s.rooms.rooms {
				if time.Since(s.rooms.rooms[room].opened) > 3*time.Hour {
					roomsToDelete = append(roomsToDelete, room)
				}
			}
			s.rooms.Unlock()

			for _, room := range roomsToDelete {
				s.deleteRoom(room)
			}
		}
	}()

	err = s.run()
	if err != nil {
		log.Error(err)
	}
	return
}

func (s *server) run() (err error) {
	log.Infof("starting TCP server on " + s.port)
	server, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return errors.Wrap(err, "Error listening on :"+s.port)
	}
	defer server.Close()
	// spawn a new goroutine whenever a client connects
	for {
		connection, err := server.Accept()
		if err != nil {
			return errors.Wrap(err, "problem accepting connection")
		}
		log.Debugf("client %s connected", connection.RemoteAddr().String())
		go func(port string, connection net.Conn) {
			c := comm.New(connection)
			room, errCommunication := s.clientCommuncation(port, c)
			if errCommunication != nil {
				log.Warnf("relay-%s: %s", connection.RemoteAddr().String(), errCommunication.Error())
			}
			for {
				// check connection
				log.Debugf("checking connection of room %s for %+v", room, c)
				deleteIt := false
				s.rooms.Lock()
				if _, ok := s.rooms.rooms[room]; !ok {
					log.Debug("room is gone")
					s.rooms.Unlock()
					return
				}
				log.Debugf("room: %+v", s.rooms.rooms[room])
				if s.rooms.rooms[room].first != nil && s.rooms.rooms[room].second != nil {
					log.Debug("rooms ready")
					s.rooms.Unlock()
					break
				} else {
					if s.rooms.rooms[room].first != nil {
						errSend := s.rooms.rooms[room].first.Send([]byte{1})
						if errSend != nil {
							log.Debug(errSend)
							deleteIt = true
						}
					}
				}
				s.rooms.Unlock()
				if deleteIt {
					s.deleteRoom(room)
					break
				}
				time.Sleep(1 * time.Second)
			}
		}(s.port, connection)
	}
}

var weakKey = []byte{1, 2, 3}

func (s *server) clientCommuncation(port string, c *comm.Comm) (room string, err error) {
	// establish secure password with PAKE for communication with relay
	B, err := pake.InitCurve(weakKey, 1, "siec", 1*time.Millisecond)
	if err != nil {
		return
	}
	Abytes, err := c.Receive()
	if err != nil {
		return
	}
	err = B.Update(Abytes)
	if err != nil {
		return
	}
	err = c.Send(B.Bytes())
	Abytes, err = c.Receive()
	if err != nil {
		return
	}
	err = B.Update(Abytes)
	if err != nil {
		return
	}
	strongKey, err := B.SessionKey()
	if err != nil {
		return
	}
	log.Debugf("strongkey: %x", strongKey)

	// receive salt
	salt, err := c.Receive()
	strongKeyForEncryption, _, err := crypt.New(strongKey, salt)
	if err != nil {
		return
	}

	// Done crypto stuff
	log.Debugf("waiting for password")
	passwordBytesEnc, err := c.Receive()
	if err != nil {
		return
	}
	passwordBytes, err := crypt.Decrypt(passwordBytesEnc, strongKeyForEncryption)
	if err != nil {
		return
	}
	if strings.TrimSpace(string(passwordBytes)) != s.password {
		err = fmt.Errorf("bad password")
		enc, _ := crypt.Decrypt([]byte(err.Error()), strongKeyForEncryption)
		c.Send(enc)
		return
	}

	// send ok to tell client relay pass is good
	banner := s.banner
	if len(banner) == 0 {
		banner = "ok"
	}

	log.Debugf("sending '%s'", banner)
	bSend, err := crypt.Encrypt([]byte(banner+"|||"+c.Connection().RemoteAddr().String()), strongKeyForEncryption)
	if err != nil {
		return
	}
	err = c.Send(bSend)
	if err != nil {
		return
	}

	// wait for client to tell me which room they want
	log.Debug("waiting for answer")
	enc, err := c.Receive()
	if err != nil {
		return
	}
	roomBytes, err := crypt.Decrypt(enc, strongKeyForEncryption)
	if err != nil {
		return
	}
	room = string(roomBytes)

	s.rooms.Lock()
	// create the room if it is new
	if _, ok := s.rooms.rooms[room]; !ok {
		// Ask for a room password
		log.Debug("waiting for room pass")
		bSend, err = crypt.Encrypt([]byte("need pass"), strongKeyForEncryption)
		if err != nil {
			return
		}
		err = c.Send(bSend)
		if err != nil {
			return
		}

		passwordBytesEnc, err = c.Receive()
		if err != nil {
			return
		}
		passwordBytes, err = crypt.Decrypt(passwordBytesEnc, strongKeyForEncryption)
		if err != nil {
			return
		}

		s.rooms.rooms[room] = roomInfo{
			first:    c,
			password: string(passwordBytes),
			opened:   time.Now(),
		}
		s.rooms.Unlock()
		// tell the client that they got the room

		bSend, err = crypt.Encrypt([]byte("ok"), strongKeyForEncryption)
		if err != nil {
			return
		}
		err = c.Send(bSend)
		if err != nil {
			log.Error(err)
			s.deleteRoom(room)
			return
		}
		log.Debugf("room %s has 1", room)
		return
	}
	if s.rooms.rooms[room].full {
		s.rooms.Unlock()
		bSend, err = crypt.Encrypt([]byte("room full"), strongKeyForEncryption)
		if err != nil {
			return
		}
		err = c.Send(bSend)
		if err != nil {
			log.Error(err)
			s.deleteRoom(room)
			return
		}
		return
	}
	// Verify password
	bSend, err = crypt.Encrypt([]byte("need pass"), strongKeyForEncryption)
	if err != nil {
		return
	}
	err = c.Send(bSend)
	if err != nil {
		return
	}
	passwordBytesEnc, err = c.Receive()
	if err != nil {
		return
	}
	passwordBytes, err = crypt.Decrypt(passwordBytesEnc, strongKeyForEncryption)
	if err != nil {
		return
	}
	if s.rooms.rooms[room].password != string(passwordBytes) {
		bSend, err = crypt.Encrypt([]byte("wrong"), strongKeyForEncryption)
		if err != nil {
			return
		}
		err = c.Send(bSend)
		if err != nil {
			return
		}
		s.rooms.Unlock()
		return
	}

	log.Debugf("room %s has 2", room)
	s.rooms.rooms[room] = roomInfo{
		first:    s.rooms.rooms[room].first,
		second:   c,
		password: s.rooms.rooms[room].password,
		opened:   s.rooms.rooms[room].opened,
		full:     true,
	}
	otherConnection := s.rooms.rooms[room].first
	s.rooms.Unlock()

	// second connection is the sender, time to staple connections
	var wg sync.WaitGroup
	var deleteIt bool
	wg.Add(1)

	// start piping
	go func(com1, com2 *comm.Comm, wg *sync.WaitGroup) {
		log.Debug("starting pipes")
		err = pipe(com1.Connection(), com2.Connection())
		wg.Done()
		deleteIt = err != nil
		log.Debug("done piping")
	}(otherConnection, c, &wg)

	// tell the sender everything is ready
	bSend, err = crypt.Encrypt([]byte("ok"), strongKeyForEncryption)
	if err != nil {
		return
	}
	err = c.Send(bSend)
	if err != nil {
		s.deleteRoom(room)
		return
	}
	wg.Wait()

	// delete room
	if deleteIt {
		s.deleteRoom(room)
	} else {
		s.rooms.Lock()
		s.rooms.rooms[room] = roomInfo{
			first:    s.rooms.rooms[room].first,
			second:   nil,
			password: s.rooms.rooms[room].password,
			opened:   s.rooms.rooms[room].opened,
			full:     false,
		}
		s.rooms.Unlock()
	}
	return
}

func (s *server) deleteRoom(room string) {
	s.rooms.Lock()
	defer s.rooms.Unlock()
	if _, ok := s.rooms.rooms[room]; !ok {
		return
	}
	log.Debugf("deleting room: %s", room)
	if s.rooms.rooms[room].first != nil {
		s.rooms.rooms[room].first.Close()
	}
	if s.rooms.rooms[room].second != nil {
		s.rooms.rooms[room].second.Close()
	}
	s.rooms.rooms[room] = roomInfo{first: nil, second: nil}
	delete(s.rooms.rooms, room)

}

// chanFromConn creates a channel from a Conn object, and sends everything it
//  Read()s from the socket to the channel.
func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte, 1)

	go func() {
		b := make([]byte, models.TCP_BUFFER_SIZE)

		for {
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				// Copy the buffer so it doesn't get changed while read by the recipient.
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				log.Debug(err)
				c <- nil
				break
			}
		}
		log.Debug("exiting")
	}()

	return c
}

// pipe creates a full-duplex pipe between the two sockets and
// transfers data from one to the other.
func pipe(host net.Conn, client net.Conn) error {
	chan1 := chanFromConn(host)
	chan2 := chanFromConn(client)

	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return fmt.Errorf("Host exited")
			}
			client.Write(b1)

		case b2 := <-chan2:
			if b2 == nil {
				return nil
			}
			host.Write(b2)
		}
	}
	return fmt.Errorf("Host exited")
}

// ConnectToTCPServer will initiate a new connection
// to the specified address, room with optional time limit
func ConnectToTCPServer(address, password, room, roomPass string, timelimit ...time.Duration) (c *comm.Comm, banner string, ipaddr string, err error) {
	if len(timelimit) > 0 {
		c, err = comm.NewConnection(address, timelimit[0])
	} else {
		c, err = comm.NewConnection(address)
	}
	if err != nil {
		return
	}

	// get PAKE connection with server to establish strong key to transfer info
	A, err := pake.InitCurve(weakKey, 0, "siec", 1*time.Millisecond)
	if err != nil {
		return
	}
	err = c.Send(A.Bytes())
	if err != nil {
		return
	}
	Bbytes, err := c.Receive()
	if err != nil {
		return
	}
	err = A.Update(Bbytes)
	if err != nil {
		return
	}
	err = c.Send(A.Bytes())
	if err != nil {
		return
	}
	strongKey, err := A.SessionKey()
	if err != nil {
		return
	}
	log.Debugf("strong key: %x", strongKey)

	strongKeyForEncryption, salt, err := crypt.New(strongKey, nil)
	// send salt
	err = c.Send(salt)
	if err != nil {
		return
	}

	log.Debug("sending password")
	bSend, err := crypt.Encrypt([]byte(password), strongKeyForEncryption)
	if err != nil {
		return
	}
	err = c.Send(bSend)
	if err != nil {
		return
	}
	log.Debug("waiting for first ok")
	enc, err := c.Receive()
	if err != nil {
		return
	}
	data, err := crypt.Decrypt(enc, strongKeyForEncryption)
	if err != nil {
		return
	}
	if !strings.Contains(string(data), "|||") {
		err = fmt.Errorf("bad response: %s", string(data))
		return
	}
	banner = strings.Split(string(data), "|||")[0]
	ipaddr = strings.Split(string(data), "|||")[1]
	log.Debug("sending room")
	bSend, err = crypt.Encrypt([]byte(room), strongKeyForEncryption)
	if err != nil {
		return
	}
	err = c.Send(bSend)
	if err != nil {
		return
	}
	log.Debug("waiting for room confirmation")
	enc, err = c.Receive()
	if err != nil {
		return
	}
	data, err = crypt.Decrypt(enc, strongKeyForEncryption)
	if err != nil {
		return
	}
	if bytes.Equal(data, []byte("need pass")) {
		bSend, err = crypt.Encrypt([]byte(roomPass), strongKeyForEncryption)
		if err != nil {
			return
		}
		err = c.Send(bSend)
		if err != nil {
			return
		}
		enc, err = c.Receive()
		if err != nil {
			return
		}
		data, err = crypt.Decrypt(enc, strongKeyForEncryption)
		if err != nil {
			return
		}
	}
	if bytes.Equal(data, []byte("wrong")) {
		err = fmt.Errorf("wromg room password")
		return
	}

	if !bytes.Equal(data, []byte("ok")) {
		err = fmt.Errorf("got bad response: %s", data)
		return
	}

	log.Debug("all set")
	return
}
