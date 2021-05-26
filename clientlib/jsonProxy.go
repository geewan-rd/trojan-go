package TrojanGO

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

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

func StartProxyWithData(jsonData []byte) error {
	var data = jsonData
	proxy, err := proxy.NewProxyFromConfigData(data, true)
	if err != nil {
		fmt.Print("error:%@", err.Error())
		log.Fatal(err)
		return err
	}
	go log.Info("StartProxyWithData")
	currentProxy = proxy
	err = proxy.Run()
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}
func StartProxyWithString(jsonData string) {
	var dat map[string]interface{}
	b := []byte(jsonData)
	if err := json.Unmarshal(b, &dat); err != nil {
		log.Fatalf("json错误：%s,json:%s", err.Error(), jsonData)
	}

	go StartProxyWithData(b)
}
