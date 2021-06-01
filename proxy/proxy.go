package proxy

import (
	"context"
	"io"
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
	"github.com/p4gefau1t/trojan-go/tunnel/adapter"
)

const Name = "PROXY"

const (
	MaxPacketSize = 1024 * 8
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
var conArr []tunnel.Conn
var lck sync.Mutex

func addConn(conn tunnel.Conn, index int) {
	printMemery()
	lck.Lock()
	conArr[index] = conn
	lck.Unlock()
}
func closeAllConn() {
	printMemery()
	// lck.Lock()
	for i, conn := range conArr {
		if conn != nil {
			conn.Close()
		}
		conArr[i] = nil
	}
	// lck.Unlock()
	gc()
}

func acceptTunnelConn(source tunnel.Server, p *Proxy) {
	var connectCount = 0
	var currentIndex = 0
	conArr = make([]tunnel.Conn, MaxCount)
	var pool *tunny.Pool
	if MaxCount > 0 {
		pool = tunny.NewFunc(MaxCount+2, func(p interface{}) interface{} {
			f := p.(func())
			f()
			return true
		})
		adapter.CanRun = func() bool {
			return canRun()
		}
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
		// if !canRun() {
		// 	inbound.Close()
		// 	time.Sleep(200 * time.Millisecond)
		// 	continue
		// }

		inboudFunc := func(inbound tunnel.Conn) {
			if MaxCount > 0 {
				index := currentIndex % MaxCount
				lck.Lock()
				if con := conArr[index]; con != nil {
					con.Close()
					log.Debugf("YETest：maxCount:(%d)超出连接限制（index:%d），关闭连接：%s", MaxCount, index, con.Metadata())
					con = nil
					gc()

				}
				lck.Unlock()
				addConn(inbound, index)
				currentIndex += 1
			}
			connectCount += 1
			log.Debugf("YETest：count:%d,连接：%s", connectCount, inbound.Metadata())

			outbound, err := p.sink.DialConn(inbound.Metadata().Address, nil)
			defer func() {
				inbound.Close()
				if outbound != nil {
					outbound.Close()
				}
				connectCount -= 1
				log.Debugf("YETest：连接关闭：%s", inbound.Metadata())
				gc()
			}()
			if err != nil {
				log.Error(common.NewError("proxy failed to dial connection").Base(err))
				return
			}

			errChan := make(chan error, 2)
			copyConn := func(a, b net.Conn) {
				_, err := io.Copy(a, b)
				errChan <- err
				gc()
				return
			}
			go copyConn(inbound, outbound)
			go copyConn(outbound, inbound)
			select {
			case err = <-errChan:
				if err != nil {
					log.Error(err)
				}
			case <-p.ctx.Done():
				log.Debug("shutting down conn relay")
				return
			}

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

var packetBound = []tunnel.PacketConn{}

func addPacketBound(conn tunnel.PacketConn) {
	lck.Lock()
	packetBound = append(packetBound, conn)
	lck.Unlock()
}
func removeAllBound() {
	// lck.Lock()
	for _, conn := range packetBound {
		conn.Close()
	}
	packetBound = []tunnel.PacketConn{}
	// lck.Unlock()
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
					log.Debug("YeTest:接收包")
					outbound, err := p.sink.DialPacket(nil)
					if err != nil {
						log.Error(common.NewError("proxy failed to dial packet").Base(err))
						return
					}
					defer outbound.Close()
					// addPacketBound(inbound)
					// addPacketBound(outbound)

					errChan := make(chan error, 2)
					copyPacket := func(a, b tunnel.PacketConn) {
						for {
							buf := make([]byte, MaxPacketSize)
							gc()
							if !canRun() {
								continue
							}
							n1, metadata, err := a.ReadWithMetadata(buf)
							if err != nil {
								errChan <- err
								return
							}
							if n1 == 0 {
								errChan <- nil
								return
							}
							n2, err1 := b.WriteWithMetadata(buf[:n1], metadata)
							if err1 != nil {
								errChan <- err1
								return
							}
							log.Debugf("YeTestaaaa读写buff:write(%d),read(%d)", n1, n2)
						}
					}
					go copyPacket(inbound, outbound)
					go copyPacket(outbound, inbound)
					select {
					case err = <-errChan:
						if err != nil {
							log.Error(err)
						}
					case <-p.ctx.Done():
						log.Debug("shutting down packet relay")
					}
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

var isAutoResetMemerying = false

func stopAllConn() {
	closeAllConn()
	time.Sleep(1 * time.Second)
}

var isResting = false

func canRun() bool {
	lck.Lock()
	var lock = !isResting
	lck.Unlock()
	return lock
}
func AutoResetMemery() {
	if isAutoResetMemerying {
		return
	}
	isAutoResetMemerying = true
	go func() {

		for {
			time.Sleep(100 * time.Millisecond)
			var info runtime.MemStats
			runtime.ReadMemStats(&info)
			var limit uint64 = 26 * 100000
			if info.Alloc > limit {
				log.Debugf("YeTest准备释放内存:alloc:%d,heapAlloc:%d", info.Alloc, info.HeapAlloc)

				lck.Lock()
				isResting = true
				var count = 0
				for {
					stopAllConn()
					removeAllBound()
					log.Debug("YeTest:关闭所有连接")
					count += 1
					var info1 runtime.MemStats
					runtime.ReadMemStats(&info1)
					log.Debugf("YeTest释放后内存:alloc:%d,heapAlloc:%d count:%d", info1.Alloc, info1.HeapAlloc, count)
					if info1.Alloc < limit*8/10 {
						log.Debugf("YeTest释放内存已达到%d", info1.Alloc)
						break
					}
					gc()
					time.Sleep(100 * time.Millisecond)
				}
				isResting = false
				lck.Unlock()
				printMemery()
			}
		}
	}()
}
func gc() {
	debug.FreeOSMemory()
}
func printMemery() {
	var info runtime.MemStats
	runtime.ReadMemStats(&info)
	log.Debugf("YeTest当前内存:alloc:%d,heapAlloc:%d", info.Alloc, info.HeapAlloc)
}
