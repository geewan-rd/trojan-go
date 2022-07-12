package TrojanGO

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	// tun2socksClient "github.com/eycorsican/go-tun2socks/clientlib/mobile"
	"github.com/p4gefau1t/trojan-go/log"
	_ "github.com/p4gefau1t/trojan-go/log/golog"
	"github.com/p4gefau1t/trojan-go/proxy"
	_ "github.com/p4gefau1t/trojan-go/proxy/client"
)

var clientMap = make(map[int]*TrojanClient)

func Start(jsonS string, tag int) {
	c := &TrojanClient{}
	clientMap[tag] = c
	go c.StartProxyWithString(jsonS)
}
func Stop(tag int) {
	c := clientMap[tag]
	if c != nil {
		c.StopProxy()
		delete(clientMap, tag)
	}
}

type TrojanClient struct {
	currentProxy *proxy.Proxy
	cachJsonData []byte
}

func (c *TrojanClient) GoStartProxy(localAddr string, localPort int, remoteAddr string, remotePort int, password string, loglevel int) {
	log.SetLogLevel(0)
	go c.StartProxy(localAddr, localPort, remoteAddr, remotePort, password, loglevel)

}
func (c *TrojanClient) StartProxy(localAddr string, localPort int, remoteAddr string, remotePort int, password string, loglevel int) error {
	jsonMap := map[string]interface{}{}
	jsonMap["run_type"] = "client"
	jsonMap["local_addr"] = localAddr
	jsonMap["local_port"] = localPort
	jsonMap["remote_addr"] = remoteAddr
	jsonMap["remote_port"] = remotePort
	jsonMap["password"] = []string{password}
	jsonMap["ssl"] = map[string]interface{}{"verify": false, "sni": ""}
	jsonMap["log_level"] = loglevel

	data, e := json.Marshal(jsonMap)
	if e != nil {
		log.Fatalf("Failed to read from stdin: %s", e.Error())
		return e
	}

	return c.StartProxyWithData(data)
}
func (c *TrojanClient) GOStartProxyWithData(jsonData []byte) {
	go c.StartProxyWithData(jsonData)
}
func (c *TrojanClient) StopProxy() {
	if c.currentProxy != nil {
		c.currentProxy.Close()
		runtime.GC()
		debug.FreeOSMemory()
	}
}

func (c *TrojanClient) StartProxyWithData(jsonData []byte) error {
	var data = jsonData
	c.cachJsonData = make([]byte, len(jsonData))
	copy(c.cachJsonData, jsonData)
	pr, err := proxy.NewProxyFromConfigData(data, true)
	if err != nil {
		fmt.Print("error:%@", err.Error())
		log.Fatal(err)
		return err
	}
	dat, err := getJson(jsonData)
	if err == nil {
		value := dat["maxConnCount"]
		if value != nil {
			proxy.MaxCount = int(value.(float64))
		}
		value = dat["MaxAlloc"]
		if value != nil {
			proxy.MaxAlloc = int(value.(float64))
		}
	}
	debug.SetGCPercent(10)
	c.currentProxy = pr
	// go func() {
	// 	time.Sleep(10 * time.Second)
	// 	go pr.Close()
	// 	time.Sleep(1 * time.Second)
	// 	go StartProxyWithData(data)
	// }()
	err = pr.Run()
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}
func (c *TrojanClient) StartProxyWithString(jsonData string) {
	b := []byte(jsonData)
	_, err := getJson(b)
	if err != nil {
		log.Fatalf("json错误：%s,json:%s", err.Error(), jsonData)
	}

	go c.StartProxyWithData(b)
}
func getJson(b []byte) (map[string]interface{}, error) {
	var dat map[string]interface{}
	if err := json.Unmarshal(b, &dat); err != nil {
		return nil, err
	}
	return dat, nil
}

func (c *TrojanClient) ReStart() {

	go c.currentProxy.Close()
	runtime.GC()
	debug.FreeOSMemory()
	time.Sleep(1000 * time.Millisecond)
	go c.StartProxyWithData(c.cachJsonData)
}

// func InputPacket(data []byte) {
// 	tun2socksClient.InputPacket(data)
// }
// func StartSocks(proxyHost string, proxyPort int, output tun2socksClient.OutputFunc) {
// 	tun2socksClient.StartSocks(proxyHost, proxyPort, output)
// }
