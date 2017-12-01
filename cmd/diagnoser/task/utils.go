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
	"bufio"
	"strings"
	"time"

	"github.com/golang/glog"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
)

const lvlmap map[string]string = map[string]string{
	"W": "Warning",
	"E": "Error",
	"F": "Fail",
}

func CheckDnsLogs(cs v1.CoreV1Interface, pods *meta_v1.PodList, tsMap map[string]time.Time) {
	for _, pod := range pods.Items {
		podName := pod.ObjectMeta.Name
		for _, containerName := range []string{"kubedns", "sidecar", "dnsmasq"} {
			req := cs.RESTClient().Get().
				Namespace("kube-system").
				Name(podName).
				Resource("pods").
				SubResource("log").
				Param("container", containerName).
				Param("timestamps", "true")

			var timestamp time.Time
			key := podName + ":" + containerName
			if timestamp, ok := tsMap[key]; ok {
				req.Param("sinceTime", timestamp.Format(time.RFC3339))
			}

			readCloser, err := req.Stream()
			if err != nil {
				return err
			}

			scanner := bufio.NewScanner(readCloser)
			for scanner.Scan() {
				line := scanner.Text()
				splitLine := strings.Fields(line)
				timestamp, _ = time.Parse(time.RFC3339, splitLine[0])
				switch lvl := string(splitLine[1][0]); lvl {
				case "E", "W", "F":
					glog.Warningf("%s detected : pod %s : container %s : %s",
						lvlmap[lvl], podName, containerName, line)
				}
			}

			if err := scanner.Err(); err != nil {
				glog.Errorf("error: %s", err)
			}

			readCloser.Close()
			tsMap[key] = timestamp
		}
	}
}
