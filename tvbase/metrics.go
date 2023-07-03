package tvbase

import (
	"net/http"
	"strconv"

	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	prometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
)

func (m *Tvbase) initMetric() {
	addr := ":" + strconv.Itoa(m.nodeCfg.Metrics.ApiPort)
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(addr, nil)
	}()

	rcmgrObs.MustRegisterWith(prometheus.DefaultRegisterer)

	var nodeInfoMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tinverseInfrasture_info",
		Help: "tinverse network version information.",
	}, []string{"version", "commit"})

	nodeInfoMetric.With(prometheus.Labels{
		"version": tvCommon.CurrentVersionNumber,
		"commit":  tvCommon.CurrentCommit,
	}).Set(1)

	prometheus.MustRegister(&NodeCollector{Node: m})
}

// NodeCollector
var (
	peersTotalMetric = prometheus.NewDesc(
		prometheus.BuildFQName("ipfs", "p2p", "peers_total"),
		"Number of connected peers",
		[]string{"transport"},
		nil,
	)
)

type NodeCollector struct {
	Node *Tvbase
}

func (NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- peersTotalMetric
}

func (c NodeCollector) Collect(ch chan<- prometheus.Metric) {
	for tr, val := range c.PeersTotalValues() {
		ch <- prometheus.MustNewConstMetric(
			peersTotalMetric,
			prometheus.GaugeValue,
			val,
			tr,
		)
	}
}

func (c NodeCollector) PeersTotalValues() map[string]float64 {
	vals := make(map[string]float64)
	if c.Node.host.Network().Peers() == nil {
		return vals
	}
	for _, peerID := range c.Node.host.Network().Peers() {
		// Each peer may have more than one connection (see for an explanation
		// https://github.com/libp2p/go-libp2p-swarm/commit/0538806), so we grab
		// only one, the first (an arbitrary and non-deterministic choice), which
		// according to ConnsToPeer is the oldest connection in the list
		// (https://github.com/libp2p/go-libp2p-swarm/blob/v0.2.6/swarm.go#L362-L364).
		conns := c.Node.host.Network().ConnsToPeer(peerID)
		if len(conns) == 0 {
			continue
		}
		tr := ""
		for _, proto := range conns[0].RemoteMultiaddr().Protocols() {
			tr = tr + "/" + proto.Name
		}
		vals[tr] = vals[tr] + 1
	}
	return vals
}
