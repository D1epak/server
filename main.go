package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
)

const multiplier = 1000000

// ProcPath is path to proc dir.
var ProcPath string = "/proc/"

type SystemInfo interface {
	DiskUsage()
}

type SystemStatus struct {
	Mem  *Memory     `json:"mem"`
	Cpu  *LoadCPU    `json:"cpu"`
	Disk *DiskStatus `json:"disk"`
}

type LoadCPU struct {
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
}
type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
}

type Memory struct {
	MemTotal     int
	MemFree      int
	MemAvailable int
}

var info = []SystemStatus{}

func DiskUsage(path string) (disk DiskStatus) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs("/", &fs)
	if err != nil {
		return
	}
	disk.All = fs.Blocks * uint64(fs.Bsize)
	disk.Free = fs.Bfree * uint64(fs.Bsize)
	disk.Used = disk.All - disk.Free
	return
}

func ReadMemoryStats() Memory {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	bufio.NewScanner(file)
	scanner := bufio.NewScanner(file)
	res := Memory{}
	for scanner.Scan() {
		key, value := parseLine(scanner.Text())
		switch key {
		case "MemTotal":
			res.MemTotal = value
		case "MemFree":
			res.MemFree = value
		case "MemAvailable":
			res.MemAvailable = value
		}
	}
	return res
}

func parseLine(raw string) (key string, value int) {
	text := strings.ReplaceAll(raw[:len(raw)-2], " ", "")
	keyValue := strings.Split(text, ":")
	return keyValue[0], toInt(keyValue[1])
}

func toInt(raw string) int {
	if raw == "" {
		return 0
	}
	res, err := strconv.Atoi(raw)
	if err != nil {
		panic(err)
	}
	return res
}

func getInfo(w http.ResponseWriter, r *http.Request) {
	info = []SystemStatus{}
	w.Header().Set("Content-Type", "application/json")
	disk := DiskUsage("/")
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
	)
	ds_all := float64(disk.All) / float64(GB)
	ds_used := float64(disk.Used) / float64(GB)
	ds_free := float64(disk.Free) / float64(GB)
	// Процессор
	procF := ProcPath + "stat"
	b, _ := ioutil.ReadFile(procF)

	lc := LoadCPU{}
	var null float64
	fmt.Sscanf(string(b), "cpu %g %g %g %g", &lc.User, &null, &lc.System, &lc.Idle)
	lc.System /= multiplier
	lc.User /= multiplier
	lc.Idle /= multiplier

	mem_total := ReadMemoryStats().MemTotal
	mem_free := ReadMemoryStats().MemFree
	mem_alailable := ReadMemoryStats().MemAvailable
	mm := Memory{MemTotal: mem_total, MemFree: mem_free, MemAvailable: mem_alailable}
	mm.MemTotal /= multiplier
	mm.MemAvailable /= multiplier
	mm.MemFree /= multiplier
	// Вывод данных
	info = append(info, SystemStatus{Mem: &Memory{mm.MemTotal, mm.MemAvailable, mm.MemTotal}, Cpu: &LoadCPU{lc.User, lc.System, lc.Idle}, Disk: &DiskStatus{uint64(ds_all), uint64(ds_used), uint64(ds_free)}})
	json.NewEncoder(w).Encode(info)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/SystemInfo", getInfo).Methods("GET")
	log.Fatal(http.ListenAndServe(":8001", r))
}
