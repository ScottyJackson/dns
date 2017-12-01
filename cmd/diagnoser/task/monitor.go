package task

import (
	"time"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
)

func RunMonitor(cs v1.CoreV1Interface) {
	for {
		dnsPods, err := cs.Pods("kube-system").List(meta_v1.ListOptions{
			LabelSelector: `k8s-app=kube-dns`})
		if err != nil {
			return err
		}

		time.Sleep(10 * time.Second)
	}
}
