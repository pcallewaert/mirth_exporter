package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

const namespace = "mirth"

var (
	logger log.Logger
	up     = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last Mirth query successful.",
		nil, nil,
	)
	channelsDeployed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channels_deployed"),
		"How many channels are deployed.",
		nil, nil,
	)
	channelsStarted = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "channels_started"),
		"How many of the deployed channels are started.",
		nil, nil,
	)
	messagesReceived = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "messages_received_total"),
		"How many messages have been received (per channel).",
		[]string{"channel"}, nil,
	)
	messagesFiltered = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "messages_filtered_total"),
		"How many messages have been filtered (per channel).",
		[]string{"channel"}, nil,
	)
	messagesQueued = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "messages_queued"),
		"How many messages are currently queued (per channel).",
		[]string{"channel"}, nil,
	)
	messagesSent = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "messages_sent_total"),
		"How many messages have been sent (per channel).",
		[]string{"channel"}, nil,
	)
	messagesErrored = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "messages_errored_total"),
		"How many messages have errored (per channel).",
		[]string{"channel"}, nil,
	)
)

type Exporter struct {
	mcCommandPath string
}

func NewExporter(mcCommandPath string) *Exporter {
	return &Exporter{
		mcCommandPath: mcCommandPath,
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- channelsDeployed
	ch <- channelsStarted
	ch <- messagesReceived
	ch <- messagesFiltered
	ch <- messagesQueued
	ch <- messagesSent
	ch <- messagesErrored
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	lines, err := e.fetchStatLines()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		logger.Error(err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)
	e.readStatus(lines, ch)
	e.readChannelStats(lines, ch)
}

func (e *Exporter) fetchStatLines() ([]string, error) {
	logger.Debug("Excuting fetchStatLines")
	// First create a query file to load in Mirth Connect CLI
	// It's not the best way by creating and removing the file all the time, but this way we are sure the content in the query file is correct (and no malicious queries can be executed)
	file, err := ioutil.TempFile(os.TempDir(), "mirth_exporter")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	defer os.Remove(file.Name())
	file.WriteString("status\nchannel stats\n")
	file.Sync()
	logger.Debugf("Using %s as query file", file.Name())
	cmd := exec.Command(e.mcCommandPath, "-s", file.Name())
	timer := time.AfterFunc(10*time.Second, func() {
		cmd.Process.Kill()
	})
	stdout, err := cmd.Output()
	timer.Stop()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(stdout), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("Unexpected output: %s", string(stdout))
	}
	logger.Debug(string(stdout))
	return lines, nil
}

func (e *Exporter) readStatus(lines []string, ch chan<- prometheus.Metric) {
	deployed := regexp.MustCompile(`^[0-9a-f-]{36}\s+[a-zA-Z]+\s+`)
	started := regexp.MustCompile(`\s+Started\s+`)
	deployedCount, startedCount := 0, 0
	for _, line := range lines {
		if deployed.MatchString(line) {
			deployedCount++
			if started.MatchString(line) {
				startedCount++
			}
		}
	}
	ch <- prometheus.MustNewConstMetric(
		channelsDeployed, prometheus.GaugeValue, float64(deployedCount),
	)
	ch <- prometheus.MustNewConstMetric(
		channelsStarted, prometheus.GaugeValue, float64(startedCount),
	)
}

func (e *Exporter) readChannelStats(lines []string, ch chan<- prometheus.Metric) {
	stat := regexp.MustCompile(`^(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.+)$`)
	for _, line := range lines {
		if stat.MatchString(line) {
			group := stat.FindStringSubmatch(line)
			channel := group[6]
			received, _ := strconv.ParseFloat(group[1], 64)
			ch <- prometheus.MustNewConstMetric(
				messagesReceived, prometheus.CounterValue, received, channel,
			)
			filtered, _ := strconv.ParseFloat(group[2], 64)
			ch <- prometheus.MustNewConstMetric(
				messagesFiltered, prometheus.CounterValue, filtered, channel,
			)
			queued, _ := strconv.ParseFloat(group[3], 64)
			ch <- prometheus.MustNewConstMetric(
				messagesQueued, prometheus.GaugeValue, queued, channel,
			)
			sent, _ := strconv.ParseFloat(group[4], 64)
			ch <- prometheus.MustNewConstMetric(
				messagesSent, prometheus.CounterValue, sent, channel,
			)
			errored, _ := strconv.ParseFloat(group[5], 64)
			ch <- prometheus.MustNewConstMetric(
				messagesErrored, prometheus.CounterValue, errored, channel,
			)
		}
	}
}

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9140",
			"Address to listen on for telemetry")
		metricsPath = flag.String("web.telemetry-path", "/metrics",
			"Path under which to expose metrics")
		mccliPath = flag.String("mccli.path", "./mccommand",
			"Path to mccommand for Mirth Connect CLI")
		loglevel = flag.String("loglevel", "INFO", "Loglevel: DEBUG, INFO, ERROR, WARN")
	)
	flag.Parse()
	logger = log.NewLogger(os.Stdout)
	logger.SetLevel(*loglevel)
	exporter := NewExporter(*mccliPath)
	prometheus.MustRegister(exporter)
	logger.Infof("Starting server: %s", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Mirth Exporter</title></head>
             <body>
             <h1>Mirth Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	logger.Fatal(http.ListenAndServe(*listenAddress, nil))
}
