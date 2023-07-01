package server

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/abcdlsj/gpipe/layer"
	"github.com/abcdlsj/gpipe/logger"
	"github.com/abcdlsj/gpipe/proxy"
	"github.com/google/uuid"
)

type Config struct {
	Port      int `toml:"port"`
	AdminPort int `toml:"admin-port"` // zero means disable admin server
}

type Server struct {
	cfg      Config
	connMap  ConnMap
	forwards []Forward
	traffics []proxy.Traffic

	m sync.RWMutex
}

type Forward struct {
	From string
	To   string

	uListener net.Listener
}

func (s *Server) addUserConn(cid string, conn net.Conn) {
	s.connMap.Add(cid, conn)
}

func (s *Server) delUserConn(cid string) {
	s.connMap.Del(cid)
}

func (s *Server) getUserConn(cid string) (net.Conn, bool) {
	return s.connMap.Get(cid)
}

func (s *Server) addForward(f Forward) {
	s.m.Lock()
	defer s.m.Unlock()
	s.forwards = append(s.forwards, f)
}

func (s *Server) delForward(to string) {
	s.m.Lock()
	defer s.m.Unlock()
	for i, ff := range s.forwards {
		if ff.To == to {
			ff.uListener.Close()
			s.forwards = append(s.forwards[:i], s.forwards[i+1:]...)
			return
		}
	}
}

func (s *Server) metric(t proxy.Traffic) {
	s.m.Lock()
	defer s.m.Unlock()
	s.traffics = append(s.traffics, t)
}

func newServer(cfg Config) *Server {
	return &Server{
		cfg: cfg,
		connMap: ConnMap{
			conns: make(map[string]net.Conn),
		},
	}
}

func parseConfig(cfgFile string) Config {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		logger.FatalF("Error reading config file: %v", err)
	}

	var cfg Config
	toml.Unmarshal(data, &cfg)

	return cfg
}

func (s *Server) Run() {
	go s.StartAdmin()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		logger.FatalF("Error listening: %v", err)
	}
	defer listener.Close()

	logger.InfoF("Server listen on port %d", s.cfg.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.InfoF("Error accepting: %v", err)
			return
		}

		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	packetType, buf, err := layer.Read(conn)
	if err != nil || buf == nil {
		logger.WarnF("Error reading from connection: %v", err)
		return
	}

	switch packetType {
	case layer.RegisterForward:
		s.handleForward(conn, buf)
	case layer.ExchangeMsg:
		s.handleMessage(conn, buf)
	case layer.CancelForward:
		s.handleCancel(layer.ParseCancelPacket(buf))
	}
}

func (s *Server) handleCancel(rPort int) {
	s.delForward(fmt.Sprintf(":%d", rPort))
	logger.InfoF("Cancel forward to port %d", rPort)
}

func (s *Server) handleForward(commuConn net.Conn, buf []byte) {
	uPort := layer.ParseRegisterPacket(buf)
	if isInvaliedPort(uPort) {
		logger.ErrorF("Invalid forward to port: %d", uPort)
		return
	}

	uListener, err := net.Listen("tcp", fmt.Sprintf(":%d", uPort))
	if err != nil {
		logger.ErrorF("Error listening: %v, port: %d", err, uPort)
		return
	}
	defer uListener.Close()

	logger.InfoF("Listening on forwarding port %d", uPort)
	s.addForward(Forward{commuConn.RemoteAddr().String(), fmt.Sprintf(":%d", uPort), uListener})
	for {
		userConn, err := uListener.Accept()
		if err != nil {
			return
		}
		logger.DebugF("Accept new user connection: %s", userConn.RemoteAddr().String())
		go func() {
			cid := uuid.NewString()[:layer.Len-1]
			s.addUserConn(cid, userConn)
			layer.ExchangeMsg.Send(commuConn, cid)
		}()
	}
}

func (s *Server) handleMessage(conn net.Conn, buf []byte) {
	rid := layer.ParseExchangePacket(buf)
	uConn, ok := s.getUserConn(rid)
	if !ok {
		return
	}

	defer s.delUserConn(rid)
	s.metric(proxy.P(conn, uConn))
}

func isInvaliedPort(port int) bool {
	return port < 0 || port > 65535
}