package tcp2

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miska12345/MiskaRFS/src/comm"
	log "github.com/miska12345/MiskaRFS/src/logger"
	"github.com/miska12345/MiskaRFS/src/models"
	"github.com/pkg/errors"
	"github.com/schollz/croc/v8/src/crypt"
	"github.com/schollz/pake"
)

type server struct {
	port     string
	banner   string
	password string
	rooms    roomMap
}

type roomInfo struct {
	host     *comm.Comm
	client   *comm.Comm
	hostChan chan []byte
	opened   time.Time
	full     bool
}

type roomMap struct {
	rooms map[string]roomInfo
	sync.Mutex
}

type roomRole struct {
	room string
	role string
}

// Run Relay server
func Run(port, debugLevel, password string) error {
	log.SetLevel(debugLevel)

	s := new(server)
	s.port = port
	s.banner = "ok"
	s.password = password
	return s.start()
}

func (s *server) start() (err error) {
	s.rooms.Lock()
	s.rooms.rooms = make(map[string]roomInfo)
	s.rooms.Unlock()

	err = s.run()
	if err != nil {
		log.Error(err)
	}
	return
}

func (s *server) run() error {
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
		go s.perClientCommunication(s.port, connection)
	}
}

func (s *server) perClientCommunication(port string, conn net.Conn) {
	c := comm.New(conn)
	key, err := s.authenticate(c)
	if err != nil {
		log.Debug(err)
		return
	}
	room, err := s.setupRoom(key, c)
	if err != nil {
		log.Debug(err)
		return
	}
	if room.role == "host" {
		for {
			// Wait until a client comes
			deleteIt := false
			s.rooms.Lock()
			if _, ok := s.rooms.rooms[room.room]; !ok {
				// Room is gone
				s.rooms.Unlock()
				log.Debug("Room is gone")
				return
			}
			if s.rooms.rooms[room.room].host != nil && s.rooms.rooms[room.room].client != nil {
				// Ready to talk!
				log.Debug("Ready to talk!")
				s.rooms.Unlock()
				s.bridge(s.rooms.rooms[room.room].host, s.rooms.rooms[room.room].client, s.rooms.rooms[room.room].hostChan, key, room.room)
				log.Debug("%v", s.rooms.rooms[room.room])
			} else if s.rooms.rooms[room.room].host != nil {
				//log.Debug("Waiting for client...")
				err = s.rooms.rooms[room.room].host.Send([]byte{1})
				if err != nil {
					deleteIt = true
					log.Warn("Host is gone")
				}
				s.rooms.Unlock()
			} else {
				deleteIt = true
				s.rooms.Unlock()
			}

			if deleteIt {
				s.deleteRoom(room.room)
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
	// Client perCom will exit
}

func (s *server) bridge(host, client *comm.Comm, hostChan chan []byte, strongKeyForEncryption []byte, room string) {
	log.Debugf("BRIDGE %v == %v", host, client)

	var wg sync.WaitGroup
	var deleteIt bool
	wg.Add(1)
	// start piping
	go func(com1, com2 *comm.Comm, wg *sync.WaitGroup) {
		log.Debug("starting pipes")
		err := pipe(host.Connection(), hostChan, client.Connection())
		wg.Done()
		deleteIt = err != nil
		log.Debug("done piping")
	}(host, client, &wg)

	err := client.Send([]byte("ok"))
	if err != nil {
		s.deleteRoom(room)
		log.Error(err)
		return
	}

	wg.Wait()
	log.Debug("finish waiting")
	if deleteIt {
		log.Debug("Room will be deleted")
		s.deleteRoom(room)
		return
	} else {
		log.Debug("Room will be preserved")
		s.rooms.Lock()
		s.rooms.rooms[room] = roomInfo{
			host:     s.rooms.rooms[room].host,
			client:   nil,
			hostChan: s.rooms.rooms[room].hostChan,
			opened:   s.rooms.rooms[room].opened,
			full:     false,
		}
		s.rooms.Unlock()
		log.Debug("Client left")
		return
	}
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
				log.Debugf("chan: %s", err)
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
func pipe(host net.Conn, chan1 chan []byte, client net.Conn) error {
	chan2 := chanFromConn(client)
	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return fmt.Errorf("Host exited")
			}
			fmt.Printf("HOST: %s\n", string(b1))
			n, err := client.Write(b1)
			log.Debugf("HOST: wrote %d %s", n, err)
		case b2 := <-chan2:
			log.Debug("Still here")
			if b2 == nil {
				// Client exist
				//close(chan1)
				return nil
			}
			fmt.Printf("CLIENT: %s\n", string(b2))
			n, err := host.Write(b2)
			log.Debugf("CLIENT: wrote %d %s", n, err)
		}
	}
}

func (s *server) deleteRoom(room string) {
	log.Debugf("Deleting room %s", room)
	s.rooms.Lock()
	if _, ok := s.rooms.rooms[room]; !ok {
		s.rooms.Unlock()
		return
	}
	if s.rooms.rooms[room].host != nil {
		s.rooms.rooms[room].host.Close()
	}
	if s.rooms.rooms[room].client != nil {
		s.rooms.rooms[room].client.Close()
	}
	delete(s.rooms.rooms, room)
	s.rooms.Unlock()
}

func (s *server) setupRoom(key []byte, conn *comm.Comm) (room *roomRole, err error) {
	log.Debug("Setup room here")
	room = new(roomRole)
	buf, err := conn.Receive()
	if err != nil {
		panic(err)
		return
	}
	buf, err = crypt.Decrypt(buf, key)
	if err != nil {
		panic(err)
		return
	}
	log.Debugf("Got room %s", string(buf))
	room.room = string(buf)
	s.rooms.Lock()
	log.Debug("got the lock")
	if _, ok := s.rooms.rooms[room.room]; !ok {
		// Room not already exist
		// This is a host
		log.Debugf("Create new room %s", room.room)
		s.rooms.rooms[room.room] = roomInfo{
			host:     conn,
			client:   nil,
			hostChan: chanFromConn(conn.Connection()),
			opened:   time.Now(),
			full:     false,
		}
		buf2, err := crypt.Encrypt([]byte("host"), key)
		if err != nil {
			s.rooms.Unlock()
			return nil, err
		}
		err = conn.Send(buf2)
		room.role = "host"
	} else {
		if s.rooms.rooms[room.room].full {
			log.Debugf("Room %s already full", room.room)
			buf, err = crypt.Encrypt([]byte("full"), key)
			if err != nil {
				s.rooms.Unlock()
				return nil, err
			}
			err = conn.Send(buf)
		} else {
			log.Debugf("Room %s has new client", room.room)
			s.rooms.rooms[room.room] = roomInfo{
				host:     s.rooms.rooms[room.room].host,
				client:   conn,
				hostChan: s.rooms.rooms[room.room].hostChan,
				opened:   s.rooms.rooms[room.room].opened,
				full:     true,
			}
			buf2, err := crypt.Encrypt([]byte("client"), key)
			if err != nil {
				s.rooms.Unlock()
				return nil, err
			}
			err = conn.Send(buf2)
			room.role = "client"
		}
	}
	s.rooms.Unlock()
	return
}

var weakKey = []byte{1, 2, 3}

func (s *server) authenticate(c *comm.Comm) (strongKeyForEncryption []byte, err error) {
	// PAKE stuff
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
	strongKeyForEncryption, _, err = crypt.New(strongKey, salt)
	if err != nil {
		return
	}
	// DONE

	// Wait for password
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
	buf, err := crypt.Encrypt([]byte("ok"), strongKeyForEncryption)
	c.Send(buf)
	return
}

func ConnectToTCPServer(address, password, room string, timelimit ...time.Duration) (c *comm.Comm, err error) {
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
	if !bytes.Equal(data, []byte("ok")) {
		log.Error("wrong password")
		return
	}

	log.Debug("Sending room info")
	data2, err := crypt.Encrypt([]byte(room), strongKeyForEncryption)
	if err != nil {
		return
	}
	c.Send(data2)

	log.Debug("Waiting for second ok")
	enc2, err := c.Receive()
	if err != nil {
		return
	}
	data, err = crypt.Decrypt(enc2, strongKeyForEncryption)
	if err != nil {
		return
	}
	if bytes.Equal(data, []byte("full")) {
		log.Error("room is full")
		return
	}
	if !bytes.Equal(data, []byte("host")) && !bytes.Equal(data, []byte("client")) {
		log.Errorf("Instead of ok received %s", data)
		return
	}

	log.Debug("All set")
	return
}
