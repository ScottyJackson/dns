package task

import (
	"bufio"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

var lvlmap = map[string]string{
	"W": "Warning",
	"E": "Error",
	"F": "Fail",
}

func CheckDnsLogs(cs v1.CoreV1Interface, pods *apiv1.PodList, tsMap map[string]string) error {
	for _, pod := range pods.Items {
		podName := pod.ObjectMeta.Name
		for _, containerName := range []string{"kubedns", "dnsmasq"} {
			req := cs.RESTClient().Get().
				Namespace("kube-system").
				Name(podName).
				Resource("pods").
				SubResource("log").
				Param("container", containerName).
				Param("timestamps", "true")

			var timestamp string
			key := podName + ":" + containerName
			if timestamp, ok := tsMap[key]; ok {
				req.Param("sinceTime", timestamp)
			}

			readCloser, err := req.Stream()
			if err != nil {
				return err
			}

			scanner := bufio.NewScanner(readCloser)
			for scanner.Scan() {
				line := scanner.Text()
				splitLine := strings.Fields(line)
				timestamp = splitLine[0]
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
			tsMap[key] = (time.Parse(time.RFC3339, timestamp) + time.Second*1).Format(time.RFC3339)
		}
	}

	return nil
}
