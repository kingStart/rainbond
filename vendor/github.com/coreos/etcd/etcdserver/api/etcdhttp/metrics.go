// Copyright 2017 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/raft"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	pathMetrics = "/metrics"
	pathHealth  = "/health"
)

// HandleMetricsHealth registers metrics and health handlers.
func HandleMetricsHealth(mux *http.ServeMux, srv *etcdserver.EtcdServer) {
	mux.Handle(pathMetrics, prometheus.Handler())
	mux.Handle(pathHealth, newHealthHandler(srv))
}

// HandlePrometheus registers prometheus handler on '/metrics'.
func HandlePrometheus(mux *http.ServeMux) {
	mux.Handle(pathMetrics, prometheus.Handler())
}

// HandleHealth registers health handler on '/health'.
func HandleHealth(mux *http.ServeMux, srv *etcdserver.EtcdServer) {
	mux.Handle(pathHealth, newHealthHandler(srv))
}

// newHealthHandler handles '/health' requests.
func newHealthHandler(srv *etcdserver.EtcdServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		h := checkHealth(srv)
		d, _ := json.Marshal(h)
		if !h.Health {
			http.Error(w, string(d), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(d)
	}
}

// TODO: remove manual parsing in etcdctl cluster-health
type health struct {
	Health bool     `json:"health"`
	Errors []string `json:"errors,omitempty"`
}

func checkHealth(srv *etcdserver.EtcdServer) health {
	h := health{Health: false}

	as := srv.Alarms()
	if len(as) > 0 {
		for _, v := range as {
			h.Errors = append(h.Errors, v.Alarm.String())
		}
		return h
	}

	if uint64(srv.Leader()) == raft.None {
		h.Errors = append(h.Errors, etcdserver.ErrNoLeader.Error())
		return h
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err := srv.Do(ctx, etcdserverpb.Request{Method: "QGET"})
	cancel()
	if err != nil {
		h.Errors = append(h.Errors, err.Error())
	}

	h.Health = err == nil
	return h
}
