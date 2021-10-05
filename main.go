package main

import (
	"context"
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type Metric struct {
	Timestamp   time.Time `json:"timestamp"`
	HostName    string    `json:"hostname"`
	Uptime      uint64    `json:"uptime"`
	CpuLoad     float64   `json:"cpu_load"`
	LocalIP     string    `json:"local_ip"`
	PublicIP    string    `json:"public_ip"`
	MemoryUsage float64   `json:"memory_usage"`
}

var Logger *logrus.Logger

func main() {
	var (
		projectID string
		keyPath   string
		interval  int
	)
	flag.StringVar(&projectID, "proj", "", "firestore project id")
	flag.StringVar(&keyPath, "key", "", "gcp json key file path")
	flag.IntVar(&interval, "i", 6, "record interval in hour")
	flag.Parse()

	Logger = logrus.New()

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID, option.WithCredentialsFile(keyPath))
	if err != nil {
		Logger.WithError(err).Fatalln("fail to init firestore client")
	}
	doc := client.Doc("/metric/host")
	metric := Metric{}

	for {
		fillMetric(&metric)
		wr, err := doc.Create(ctx, metric)
		if err != nil {
			Logger.WithError(err).Error("fail to create record")
		}
		Logger.WithField("result", wr).Info("metric created")
		time.Sleep(time.Duration(interval) * time.Hour)
	}
}

func fillMetric(m *Metric) {
	m.Timestamp = time.Now()
	m.PublicIP = getPublicIP()
	m.LocalIP = getLocalIP()

	if memory, err := mem.VirtualMemory(); err != nil {
		Logger.WithError(err).Error("memory stat read error")
		m.MemoryUsage = -1
	} else {
		m.MemoryUsage = memory.UsedPercent
	}
	if h, err := host.Info(); err != nil {
		Logger.WithError(err).Error("host stat read error")
		m.Uptime = 0
		m.HostName = "error"
	} else {
		m.Uptime = h.Uptime
		m.HostName = h.Hostname
	}
	if a, err := load.Avg(); err != nil {
		Logger.WithError(err).Error("cpu stat read error")
		m.CpuLoad = -1
	} else {
		m.CpuLoad = a.Load15
	}
}

func getPublicIP() string {
	res, err := http.Get("https://api.ipify.org")
	if err != nil {
		Logger.WithError(err).Errorln("public ip api request error")
		return "error"
	}
	ip, err := ioutil.ReadAll(res.Body)
	if err != nil {
		Logger.WithError(err).Errorln("public ip api read error")
		return "error"
	}
	return string(ip)
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		Logger.WithError(err).Errorln("local ip read error")
		return "error"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
