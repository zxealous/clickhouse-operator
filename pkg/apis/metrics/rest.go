// Copyright 2019 Altinity Ltd and/or its affiliates. All rights reserved.
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

package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartMetricsREST start Prometheus metrics exporter in background
func StartMetricsREST(
	chAccess *CHAccessInfo,

	metricsAddress string,
	metricsPath string,

	chiListAddress string,
	chiListPath string,
) *Exporter {
	// Initializing Prometheus Metrics Exporter
	glog.V(1).Infof("Starting metrics exporter at '%s%s'\n", metricsAddress, metricsPath)
	exporter = NewExporter(chAccess)
	prometheus.MustRegister(exporter)

	http.Handle(metricsPath, promhttp.Handler())
	http.Handle(chiListPath, exporter)

	go http.ListenAndServe(metricsAddress, nil)
	if metricsAddress != chiListAddress {
		go http.ListenAndServe(chiListAddress, nil)
	}

	return exporter
}

func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/chi" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		e.getWatchedCHI(w, r)
	case "POST":
		e.addWatchedCHI(w, r)
	case "DELETE":
		e.deleteWatchedCHI(w, r)
	default:
		fmt.Fprintf(w, "Sorry, only GET, POST and DELETE methods are supported.")
	}
}

func (e *Exporter) getWatchedCHI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(exporter.chInstallations.Slice())
}

func (e *Exporter) addWatchedCHI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chi := &WatchedChi{}
	if err := json.NewDecoder(r.Body).Decode(chi); err == nil {
		if !chi.empty() {
			// All is OK, CHI seems to be valid
			exporter.addToWatched(chi)
			return
		}
	}

	http.Error(w, "Unable to parse CHI.", http.StatusNotAcceptable)
}

func (e *Exporter) deleteWatchedCHI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chi := &WatchedChi{}
	if err := json.NewDecoder(r.Body).Decode(chi); err == nil {
		if !chi.empty() {
			// All is OK, CHI seems to be valid
			exporter.enqueueToRemoveFromWatched(chi)
			return
		}
	}

	http.Error(w, "Unable to parse CHI.", http.StatusNotAcceptable)
}

func MakeRESTCall(chi *WatchedChi, op string) error {
	url := "http://127.0.0.1:8888/chi"

	json, err := json.Marshal(chi)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(op, url, bytes.NewBuffer(json))
	if err != nil {
		return err
	}
	//req.SetBasicAuth(s.Username, s.Password)
	_, err = doRequest(req)

	return err
}

func UpdateWatchREST(namespace, chiName string, hostnames []string) error {
	chi := &WatchedChi{
		Namespace: namespace,
		Name:      chiName,
		Hostnames: hostnames,
	}
	return MakeRESTCall(chi, "POST")
}

func DeleteWatchREST(namespace, chiName string) error {
	chi := &WatchedChi{
		Namespace: namespace,
		Name:      chiName,
		Hostnames: []string{},
	}
	return MakeRESTCall(chi, "DELETE")
}

func doRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("NON 200 status code: %s", body)
	}

	return body, nil
}
