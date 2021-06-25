package proxy

import (
	"context"
	"math/rand"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/Jeffail/tunny"
	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/tunnel"
)

const Name = "PROXY"

const (
	MaxPacketSize = 1024 * 12
)

// Proxy relay connections and packets
type Proxy struct {
	sources []tunnel.Server
	sink    tunnel.Client
	ctx     context.Context
	cancel  context.CancelFunc
}

func (p *Proxy) Run() error {
	p.relayConnLoop()
	p.relayPacketLoop()
	<-p.ctx.Done()
	return nil
}

func (p *Proxy) Close() error {
	p.cancel()
	p.sink.Close()
	for _, source := range p.sources {
		source.Close()
	}
	return nil
}

func (p *Proxy) relayConnLoop() {
	for _, source := range p.sources {
		go acceptTunnelConn(source, p)
	}
}

var MaxCount = 0

func acceptTunnelConn(source tunnel.Server, p *Proxy) {
	var currentIndex = 0
	conArr := make([]tunnel.Conn, MaxCount)
	var lck sync.Mutex
	addConn := func(conn tunnel.Conn, index int) {
		if MaxCount > 0 {
			lck.Lock()
			var alloc = getMemAlloc()
			if alloc > 1000000 {
				for i, con := range conArr {
					if con != nil {
						con.Close()
						conArr[i] = nil
					}

				}
				time.Sleep(500 * time.Microsecond)
			} else if con := conArr[index]; con != nil {
				con.Close()
			}
			conArr[index] = conn
			lck.Unlock()
		}
	}
	var pool *tunny.Pool
	if MaxCount > 0 {
		// pool = tunny.NewFunc(MaxCount+2, func(p interface{}) interface{} {
		// 	f := p.(func())
		// 	f()
		// 	return true
		// })
	}

	for {
		inbound, err := source.AcceptConn(nil)
		if err != nil {
			select {
			case <-p.ctx.Done():
				log.Debug("exiting")
				return
			default:
			}
			log.Error(common.NewError("failed to accept connection").Base(err))
			continue
		}
		if MaxCount > 0 {
			index := currentIndex % MaxCount
			addConn(inbound, index)
			currentIndex += 1
		}
		inboudFunc := func(inbound tunnel.Conn) {

			outbound, err := p.sink.DialConn(inbound.Metadata().Address, nil)
			defer func() {
				inbound.Close()
				if outbound != nil {
					outbound.Close()
				}
				gc()
			}()
			if err != nil {
				log.Error(common.NewError("proxy failed to dial connection").Base(err))
				return
			}
			copyConn := func(a, b net.Conn) {
				buf := make([]byte, 512*8)
				for {
					select {
					case <-p.ctx.Done():
						log.Debug("shutting down conn relay")
						return
					default:
					}
					defer gc()
					var n int
					n, err = b.Read(buf)
					if err != nil {
						break
					}
					_, err = a.Write(buf[:n])
					if err != nil {
						break
					}
				}
			}
			go copyConn(inbound, outbound)
			copyConn(outbound, inbound)

			log.Debug("conn relay ends")
		}
		if pool == nil {
			go inboudFunc(inbound)
		} else {
			go pool.Process(func() {
				inboudFunc(inbound)
			})
		}
	}
}

func (p *Proxy) relayPacketLoop() {
	for _, source := range p.sources {
		go func(source tunnel.Server) {
			for {
				inbound, err := source.AcceptPacket(nil)
				if err != nil {
					select {
					case <-p.ctx.Done():
						log.Debug("exiting")
						return
					default:
					}
					log.Error(common.NewError("failed to accept packet").Base(err))
					continue
				}

				go func(inbound tunnel.PacketConn) {
					defer inbound.Close()
					outbound, err := p.sink.DialPacket(nil)
					if err != nil {
						log.Error(common.NewError("proxy failed to dial packet").Base(err))
						return
					}
					defer outbound.Close()

					copyPacket := func(a, b tunnel.PacketConn) {
						buf := make([]byte, MaxPacketSize)
						for {
							select {
							case <-p.ctx.Done():
								log.Debug("shutting down packet relay")
							default:
							}
							defer gc()
							n1, metadata, err := a.ReadWithMetadata(buf)

							if err != nil {
								return
							}
							if n1 == 0 {

								return
							}
							_, err1 := b.WriteWithMetadata(buf[:n1], metadata)
							if err1 != nil {
								return
							}

						}
					}
					go copyPacket(inbound, outbound)
					copyPacket(outbound, inbound)

					log.Debug("packet relay ends")
				}(inbound)
			}
		}(source)
	}
}

func NewProxy(ctx context.Context, cancel context.CancelFunc, sources []tunnel.Server, sink tunnel.Client) *Proxy {
	return &Proxy{
		sources: sources,
		sink:    sink,
		ctx:     ctx,
		cancel:  cancel,
	}
}

type Creator func(ctx context.Context) (*Proxy, error)

var creators = make(map[string]Creator)

func RegisterProxyCreator(name string, creator Creator) {
	creators[name] = creator
}

func NewProxyFromConfigData(data []byte, isJSON bool) (*Proxy, error) {
	// create a unique context for each proxy instance to avoid duplicated authenticator
	ctx := context.WithValue(context.Background(), Name+"_ID", rand.Int())
	var err error
	if isJSON {
		ctx, err = config.WithJSONConfig(context.Background(), data)
		if err != nil {
			return nil, err
		}
	} else {
		ctx, err = config.WithYAMLConfig(context.Background(), data)
		if err != nil {
			return nil, err
		}
	}
	cfg := config.FromContext(ctx, Name).(*Config)
	sValue := strings.ToUpper(cfg.RunType)
	// RegisterProxyCreator(sValue, creators)
	create, ok := creators[sValue]
	if !ok {
		return nil, common.NewError("unknown proxy type: " + cfg.RunType)
	}
	log.SetLogLevel(log.LogLevel(cfg.LogLevel))
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, common.NewError("failed to open log file").Base(err)
		}
		log.SetOutput(file)
	}
	return create(ctx)
}

func gc() {
	debug.FreeOSMemory()
}
func getMemAlloc() int {
	var info runtime.MemStats
	runtime.ReadMemStats(&info)
	return int(info.Alloc)
}
func printMemery() {
	var info runtime.MemStats
	runtime.ReadMemStats(&info)
	log.Debugf("YeTest当前内存:alloc:%d,heapAlloc:%d", info.Alloc, info.HeapAlloc)
}
