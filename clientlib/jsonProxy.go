package TrojanGO

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	// tun2socksClient "github.com/eycorsican/go-tun2socks/clientlib/mobile"
	"github.com/p4gefau1t/trojan-go/log"
	_ "github.com/p4gefau1t/trojan-go/log/golog"
	"github.com/p4gefau1t/trojan-go/proxy"
	_ "github.com/p4gefau1t/trojan-go/proxy/client"
)

func GoStartProxy(localAddr string, localPort int, remoteAddr string, remotePort int, password string) {
	log.SetLogLevel(0)
	go StartProxy(localAddr, localPort, remoteAddr, remotePort, password)

}
func StartProxy(localAddr string, localPort int, remoteAddr string, remotePort int, password string) error {
	jsonMap := map[string]interface{}{}
	jsonMap["run_type"] = "client"
	jsonMap["local_addr"] = localAddr
	jsonMap["local_port"] = localPort
	jsonMap["remote_addr"] = remoteAddr
	jsonMap["remote_port"] = remotePort
	jsonMap["password"] = []string{password}
	jsonMap["ssl"] = map[string]interface{}{"verify": false, "sni": ""}
	jsonMap["log_level"] = 0

	data, e := json.Marshal(jsonMap)
	if e != nil {
		log.Fatalf("Failed to read from stdin: %s", e.Error())
		return e
	}

	return StartProxyWithData(data)
}
func GOStartProxyWithData(jsonData []byte) {
	go StartProxyWithData(jsonData)
}
func StopProxy() {
	if currentProxy != nil {
		currentProxy.Close()
		runtime.GC()
		debug.FreeOSMemory()
	}
}
func TestStopProxy() {
	StopProxy()
	go func() {
		time.Sleep(5 * time.Second)
		log.Debug("停止后，3秒强制退出进程")
		runtime.Goexit()
		os.Exit(0)
	}()
}

var currentProxy *proxy.Proxy
var cachJsonData []byte

func StartProxyWithData(jsonData []byte) error {
	var data = jsonData
	cachJsonData = make([]byte, len(jsonData))
	copy(cachJsonData, jsonData)
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
		value = dat["DebugAlloc"]
		if value != nil {
			if value.(bool) == true {
				debugShowAlloc()
			}
		}
		value = dat["autoResetMemery"]
		if value != nil {
			if value.(bool) == true {
				proxy.AutoResetMemery()
			}
		}
	}
	debug.SetGCPercent(10)
	go log.Info("StartProxyWithData")
	currentProxy = pr
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
func StartProxyWithString(jsonData string) {
	b := []byte(jsonData)
	_, err := getJson(b)
	if err != nil {
		log.Fatalf("json错误：%s,json:%s", err.Error(), jsonData)
	}

	go StartProxyWithData(b)
}
func getJson(b []byte) (map[string]interface{}, error) {
	var dat map[string]interface{}
	if err := json.Unmarshal(b, &dat); err != nil {
		return nil, err
	}
	return dat, nil
}

var isDebugShowAlloc = false

func debugShowAlloc() {
	if isDebugShowAlloc {
		return
	}
	isDebugShowAlloc = true
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			var info runtime.MemStats
			runtime.ReadMemStats(&info)
			log.Debugf("YeTest:alloc:%d,heapAlloc:%d", info.Alloc, info.HeapAlloc)
		}
	}()
}
func ReStart() {

	go currentProxy.Close()
	runtime.GC()
	debug.FreeOSMemory()
	time.Sleep(1000 * time.Millisecond)
	go StartProxyWithData(cachJsonData)
}

// func InputPacket(data []byte) {
// 	tun2socksClient.InputPacket(data)
// }
// func StartSocks(proxyHost string, proxyPort int, output tun2socksClient.OutputFunc) {
// 	tun2socksClient.StartSocks(proxyHost, proxyPort, output)
// }
