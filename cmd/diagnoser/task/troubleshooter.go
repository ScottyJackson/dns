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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/dns/cmd/diagnoser/flags"
)

func RunTroubleshooter(opt *flags.Options, cs v1.CoreV1Interface) error {
	// Step 1: check that dns-pods are up / get number of restarts
	glog.Info("Checking that kube-dns pods are up...")
	dnsPods, err := cs.Pods("kube-system").List(meta_v1.ListOptions{
		LabelSelector: `k8s-app=kube-dns`})
	if err != nil {
		return err
	}

	nPods := len(dnsPods.Items)
	if nPods > 0 {
		glog.Infof("Total DNS pods: %d", len(dnsPods.Items))
	} else {
		glog.Warningf("No DNS pods are running!")
	}

	// Step 2: search through logs for log level > I
	glog.Info("Parsing kube-dns logs for suspicious logs...")
	CheckDnsLogs(cs, dnsPods, make(map[string]time.Time))

	// Step 3: Verify that the dns-service is up
	glog.Info("Checking kube-dns Service...")
	if dnsSvc, err := cs.Services("kube-system").Get("kube-dns", meta_v1.GetOptions{}); err == nil {
		clusterIP := dnsSvc.Spec.ClusterIP

		externalIPs := dnsSvc.Spec.ExternalIPs
		var extIP string
		if len(externalIPs) == 0 {
			extIP = "<none>"
		} else {
			extIP = strings.Join(externalIPs, ",")
		}

		ports := ""
		for _, port := range dnsSvc.Spec.Ports {
			ports += fmt.Sprintf("%s/%s,", port.Protocol, strconv.Itoa(int(port.Port)))
		}
		ports = ports[:len(ports)-1]

		glog.Infof("Found kube-dns Service: CLUSTER-IP: %s, EXTERNAL-IP: %s, PORT(S): %s", clusterIP, extIP, ports)
	} else {
		glog.Warning("kube-dns Service not found!")
	}

	// Step 4: Verify that endpoints are exposed
	glog.Info("Verifying that endpoints for kube-dns are exposed...")
	if eps, err := cs.Endpoints("kube-system").Get("kube-dns", meta_v1.GetOptions{}); err == nil {
		for _, subset := range eps.Subsets {
			epSl := make([]string, 0)
			for _, addr := range subset.Addresses {
				epSl = append(epSl, addr.IP)
			}
			glog.Infof("Found endpoints: %s", strings.Join(epSl, ","))
		}
	} else {
		glog.Warning("kube-dns endpoints not found!")
	}

	return nil
}
