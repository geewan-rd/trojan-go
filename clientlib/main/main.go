package main

import (
	// "log"

	TrojanGO "github.com/p4gefau1t/trojan-go/clientlib"

	// _ "github.com/p4gefau1t/trojan-go/log/golog"
	"net/http"
	_ "net/http/pprof"
)

func main() {

	go TrojanGO.StartProxy("127.0.0.1", 7777, "47.242.176.86", 443, "fobwifi")

	http.ListenAndServe("0.0.0.0:6060", nil)

	// var first runtime.MemStats
	// runtime.ReadMemStats(&first)
	// log.SetLogLevel(0)
	// debug.SetGCPercent(1)
	// go func() {
	// 	time.Sleep(5 * time.Second)
	// 	TrojanGO.StopProxy()
	// }()
	// for {
	// 	time.Sleep(1 * time.Second)
	// 	var current runtime.MemStats
	// 	runtime.ReadMemStats(&current)
	// 	firstO := current.HeapAlloc - first.HeapAlloc
	// 	log.Info("firt0:%d", byteToMB(firstO))
	// 	runtime.GC()
	// 	debug.FreeOSMemory()

	// }

}

func byteToMB(m uint64) float64 {
	return float64(m) / 1024 / 1024
}
