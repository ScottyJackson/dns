/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package task

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/golang/glog"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
)

func RunMonitor(cs v1.CoreV1Interface) error {
	// contains mapping of <pod:container>:<timestamp> or <pod:metric>:<timestamp>
	// used for collecting latest results for logs and metrics
	tsMap := make(map[string]string)
	// baseURI for heapster API requests
	baseHeapsterURI := "http://localhost:8001/api/v1/proxy/namespaces/kube-system/services/heapster/api/v1/model/namespaces/kube-system"
	// slice of wanted pod level metrics
	podMetrics := []string{
		"network/rx_rate",
		"network/rx_errors",
		"network/tx_rate",
		"network/tx_errors",
		"cpu/usage_rate",
		"memory/usage"}

	time.Sleep(10 * time.Second)

	for totalRunTime := 0 * time.Second; totalRunTime < 3*time.Hour; {

		dnsPods, err := cs.Pods("kube-system").List(meta_v1.ListOptions{LabelSelector: `k8s-app=kube-dns`})
		if err != nil {
			return err
		}

		for _, pod := range dnsPods.Items {
			glog.Infof("Pod: %s", pod.Name)

			for _, metric := range podMetrics {
				podURI := fmt.Sprintf("/pods/%s/metrics/%s", pod.Name, metric)
				key := pod.Name + metric
				if timestamp, ok := tsMap[key]; ok {
					podURI += fmt.Sprintf("?start=%s", timestamp)
				}
				reqURI := baseHeapsterURI + podURI
				metricResp, err := makeRequest(reqURI)
				if err != nil {
					glog.Error(err)
					continue
				}

				for _, record := range metricResp["metrics"].([]interface{}) {
					record, _ := record.(map[string]interface{})
					glog.Infof("[%v] %s: %v", record["timestamp"], metric, record["value"])
				}

				tsMap[key] = metricResp["latestTimestamp"].(string)
			}

			glog.Infof("Container Restart Counts")
			for _, container := range pod.Status.ContainerStatuses {
				glog.Infof("%s: %d", container.Name, container.RestartCount)
			}
		}

		glog.Infof("Checking Container Logs")
		err = CheckDnsLogs(cs, dnsPods, tsMap)

		services, err := cs.Services("").List(meta_v1.ListOptions{})
		if err != nil {
			return err
		}

		for _, service := range services.Items {
			ns := service.ObjectMeta.Namespace

			srvcName := service.ObjectMeta.Name
			if len(ns) > 0 {
				srvcName += fmt.Sprintf(".%s", ns)
			}

			glog.Infof("DNS Lookups for %s", srvcName)

			start := time.Now()
			cname, err := net.LookupCNAME(srvcName)
			elapsed := time.Since(start)
			if err != nil {
				glog.Errorf("%s -- took %v", err, elapsed)
			}
			glog.Infof("CNAME: %s -- took %v", cname, elapsed)

			start = time.Now()
			addrs, err := net.LookupHost(srvcName)
			elapsed = time.Since(start)
			if err != nil {
				glog.Errorf("%s -- took %v", err, elapsed)
			}
			glog.Infof("ADDRS: %+v -- took %v", addrs, elapsed)

			start = time.Now()
			ips, err := net.LookupIP(srvcName)
			elapsed = time.Since(start)
			if err != nil {
				glog.Errorf("%s -- took %v", err, elapsed)
			}
			glog.Infof("IPS: %+v -- took %v", ips, elapsed)
		}

		time.Sleep(10 * time.Second)
		totalRunTime += 10 * time.Second
	}

	return nil
}

func makeRequest(reqURI string) (data map[string]interface{}, err error) {
	resp, err := http.Get(reqURI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	return
}
