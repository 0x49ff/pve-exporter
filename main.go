package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

const promNamespace = "pve"

var (
	tr = http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client = http.Client{Transport: &tr}

	up = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "up"),
		"Was the last query is successful.",
		nil, nil,
	)
	cpu = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "cpu_usage"),
		"CPU Usage",
		[]string{"vm_id", "vm_name"}, nil,
	)
	netIn = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "net_in"),
		"Incoming network traffic",
		[]string{"vm_id", "vm_name"}, nil,
	)
	netOut = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "net_out"),
		"Outgoing network traffic",
		[]string{"vm_id", "vm_name"}, nil,
	)
	memUsage = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "mem_usage"),
		"VM memory usage",
		[]string{"vm_id", "vm_name"}, nil,
	)
	memMax = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "mem_max"),
		"VM memory max",
		[]string{"vm_id", "vm_name"}, nil,
	)
	datastoreTotal = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "datastore_total"),
		"Total datastore capacity",
		[]string{"storage"}, nil,
	)
	datastoreAvail = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "datastore_avail"),
		"Available datastore capacity",
		[]string{"storage"}, nil,
	)
	datastoreUsed = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, "", "datastore_used"),
		"Used datastore capacity",
		[]string{"storage"}, nil,
	)
)

type Exporter struct {
	endpoint, authorizationHeader string
}

type Datastore struct {
	Status    int    `json:"active,omitempty"`  // status
	Name      string `json:"storage,omitempty"` // name of storage
	Available int    `json:"avail,omitempty"`   // avail capacity
	Used      int    `json:"used,omitempty"`    // used capacity
	Total     int    `json:"total,omitempty"`   // total capacity
}

type VirtualMachine struct {
	CPU       float64 `json:"cpu"`
	CPUs      uint64
	Disk      uint64
	DiskRead  uint64 `json:"diskread"`
	DiskWrite uint64 `json:"diskwrite"`
	MaxDisk   uint64 `json:"maxdisk"`
	MaxMem    uint64 `json:"maxmem"`
	Mem       uint64 `json:"mem"`
	Name      string `json:"name"`
	NetIn     uint64 `json:"netin"`
	NetOut    uint64 `json:"netout"`
	PID       uint64
	Status    string `json:"status"`
	Uptime    uint64
	VMID      uint64 `json:"vmid"`
}

type VirtualMachineResponse struct {
	Data []VirtualMachine `json:"data"`
}

type DatastoreResponse struct {
	Data []Datastore `json:"data"`
}

func NewExporter(endpoint string, apiToken string, apiSecret string) *Exporter {
	return &Exporter{
		endpoint:            fmt.Sprintf("%v/api2/json", endpoint),
		authorizationHeader: fmt.Sprintf("PVEAPIToken=%v=%v", apiToken, apiSecret),
	}
}
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- cpu
	ch <- netIn
	ch <- netOut
	ch <- memUsage
	ch <- memMax
	ch <- datastoreTotal
	ch <- datastoreAvail
	ch <- datastoreUsed
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	err := e.collectFromAPI(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		log.Printf("%v", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)

}

func (e *Exporter) collectFromAPI(ch chan<- prometheus.Metric) error {
	err := e.collectVMmetrics(ch)
	if err != nil {
		return err
	}

	err = e.collectStorageMetrics(ch)
	if err != nil {
		return err
	}

	return nil
}

func (e *Exporter) collectVMmetrics(ch chan<- prometheus.Metric) error {
	req, err := http.NewRequest(http.MethodGet, e.endpoint+"/nodes/localhost/qemu/", nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", e.authorizationHeader)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var vmResponse VirtualMachineResponse
	err = json.Unmarshal(resBody, &vmResponse)
	if err != nil {
		return err
	}

	for _, vm := range vmResponse.Data {
		id := strconv.Itoa(int(vm.VMID))

		ch <- prometheus.MustNewConstMetric(
			netIn, prometheus.GaugeValue, float64(vm.NetIn), id, vm.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			netOut, prometheus.GaugeValue, float64(vm.NetOut), id, vm.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			memMax, prometheus.GaugeValue, float64(vm.MaxMem), id, vm.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			memUsage, prometheus.GaugeValue, float64(vm.Mem), id, vm.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			cpu, prometheus.GaugeValue, vm.CPU, id, vm.Name,
		)
	}

	return nil
}

func (e *Exporter) collectStorageMetrics(ch chan<- prometheus.Metric) error {
	req, err := http.NewRequest(http.MethodGet, e.endpoint+"/nodes/localhost/storage/", nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", e.authorizationHeader)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var storageResponse DatastoreResponse
	err = json.Unmarshal(resBody, &storageResponse)
	if err != nil {
		return err
	}

	for _, s := range storageResponse.Data {
		ch <- prometheus.MustNewConstMetric(
			datastoreTotal, prometheus.GaugeValue, float64(s.Total), s.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			datastoreAvail, prometheus.GaugeValue, float64(s.Available), s.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			datastoreUsed, prometheus.GaugeValue, float64(s.Used), s.Name,
		)
	}

	return nil
}

func main() {

	// flags
	endpoint := flag.String("endpoint", "",
		"PVE endpoint")
	apiToken := flag.String("apitoken", "",
		"PVE API token (user@realm!name)")
	apiSecret := flag.String("apisecret", "",
		"PVE API secret")
	address := flag.String("address", ":8000",
		"Address on which to expose metrics")
	path := flag.String("path", "/metrics",
		"Metrics path (/path)")

	// check env
	config := map[string]*string{
		"PVE_ENDPOINT":   endpoint,
		"PVE_API_TOKEN":  apiToken,
		"PVE_API_SECRET": apiSecret,
		"PVE_ADDRESS":    address,
		"PVE_PATH":       path,
	}

	for key, value := range config {
		if envValue := os.Getenv(key); envValue != "" {
			*value = envValue
		}
	}

	flag.Parse()

	exporter := NewExporter(*endpoint, *apiToken, *apiSecret)
	r := prometheus.NewRegistry()
	r.MustRegister(exporter)

	http.Handle(*path, promhttp.HandlerFor(r, promhttp.HandlerOpts{}))
	slog.Info(fmt.Sprintf("Listening on %v%v", *address, *path))
	http.ListenAndServe(*address, nil)
}