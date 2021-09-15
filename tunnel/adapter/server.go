package adapter

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/tunnel"
	"github.com/p4gefau1t/trojan-go/tunnel/freedom"
	"github.com/p4gefau1t/trojan-go/tunnel/http"
	"github.com/p4gefau1t/trojan-go/tunnel/socks"
)

type Server struct {
	tcpListener net.Listener
	udpListener net.PacketConn
	socksConn   chan tunnel.Conn
	httpConn    chan tunnel.Conn
	socksLock   sync.RWMutex
	nextSocks   bool
	ctx         context.Context
	cancel      context.CancelFunc
}

func (s *Server) acceptConnLoop() {
	for {
		conn, err := s.tcpListener.Accept()

		if err != nil {
			select {
			case <-s.ctx.Done():
				log.Debug("exiting")
				return
			default:
				continue
			}
		}
		log.Debugf("YeTest:socks5 src %s,remote:%s", conn.LocalAddr(), conn.RemoteAddr())
		rewindConn := common.NewRewindConn(conn)
		rewindConn.SetBufferSize(16)
		buf := [3]byte{}
		_, err = rewindConn.Read(buf[:])
		rewindConn.Rewind()
		rewindConn.StopBuffering()
		if err != nil {
			log.Error(common.NewError("failed to detect proxy protocol type").Base(err))
			continue
		}
		s.socksLock.RLock()
		if buf[0] == 5 && s.nextSocks {
			s.socksLock.RUnlock()

			timer := time.NewTimer(2 * time.Second)
			select {
			case s.socksConn <- &freedom.Conn{
				Conn: rewindConn,
			}:
				log.Debug("socks5 connection")
			case <-timer.C:
				conn := <-s.socksConn
				log.Debugf("scoks5 timeout current:%s,clear:%s", rewindConn.LocalAddr(), conn.LocalAddr())
			}

		} else {
			s.socksLock.RUnlock()
			log.Debug("http connection")
			s.httpConn <- &freedom.Conn{
				Conn: rewindConn,
			}
		}
		log.Debug("next AcceptConn")
	}
}

func (s *Server) AcceptConn(overlay tunnel.Tunnel) (tunnel.Conn, error) {
	log.Debug("AcceptConn")
	if _, ok := overlay.(*http.Tunnel); ok {
		select {
		case conn := <-s.httpConn:
			log.Debug("AcceptConn https")
			return conn, nil
		case <-s.ctx.Done():
			return nil, common.NewError("adapter closed")
		}
	} else if _, ok := overlay.(*socks.Tunnel); ok {
		log.Debug("AcceptConn lock")
		s.socksLock.Lock()
		s.nextSocks = true
		s.socksLock.Unlock()
		log.Debug("AcceptConn unlock")
		select {
		case conn := <-s.socksConn:
			log.Debug("AcceptConn socks")
			return conn, nil
		case <-s.ctx.Done():
			return nil, common.NewError("adapter closed")
		}
	} else {
		panic("invalid overlay")
	}
}

func (s *Server) AcceptPacket(tunnel.Tunnel) (tunnel.PacketConn, error) {
	return &freedom.PacketConn{
		UDPConn: s.udpListener.(*net.UDPConn),
	}, nil
}

func (s *Server) Close() error {
	s.cancel()
	s.tcpListener.Close()
	return s.udpListener.Close()
}

func NewServer(ctx context.Context, _ tunnel.Server) (*Server, error) {
	cfg := config.FromContext(ctx, Name).(*Config)
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	addr := tunnel.NewAddressFromHostPort("tcp", cfg.LocalHost, cfg.LocalPort)
	tcpListener, err := net.Listen("tcp", addr.String())
	if err != nil {
		cancel()
		return nil, common.NewError("adapter failed to create tcp listener").Base(err)
	}
	udpListener, err := net.ListenPacket("udp", addr.String())
	if err != nil {
		cancel()
		return nil, common.NewError("adapter failed to create tcp listener").Base(err)
	}
	server := &Server{
		tcpListener: tcpListener,
		udpListener: udpListener,
		socksConn:   make(chan tunnel.Conn, 32),
		httpConn:    make(chan tunnel.Conn, 32),
		ctx:         ctx,
		cancel:      cancel,
	}
	log.Info("adapter listening on tcp/udp:", addr)
	go server.acceptConnLoop()
	return server, nil
}
