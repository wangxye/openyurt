/*
Copyright 2024 The OpenYurt Authors.

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

package util

import (
	"context"
	"fmt"
	"net"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openyurtio/openyurt/pkg/projectinfo"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	PODHOSTNAME  = "/etc/hostname"
	PODNAMESPACE = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func GetNodePool(cfg *rest.Config, nodeName string) (string, error) {
	var nodePool string
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nodePool, err
	}
	node, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nodePool, fmt.Errorf("not found node %s: %v", nodeName, err)
	}
	nodePool, ok := node.Labels[projectinfo.GetNodePoolLabel()]
	if !ok {
		return nodePool, fmt.Errorf("node %s doesn't add to a nodepool", node.GetName())
	}
	return nodePool, err
}

func GetNodeInterface() (string, error) {
	// TODO: update interface when interfaces change
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("get network interfaces error: %v", err)
	}

	excludedPrefixes := []string{"lo", "docker", "flannel", "cbr"}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		excluded := false
		for _, prefix := range excludedPrefixes {
			if strings.HasPrefix(iface.Name, prefix) {
				excluded = true
				break
			}
		}
		if !excluded {
			return iface.Name, nil
		}
	}

	return "", fmt.Errorf("no available network interface found")
}

func ValidInterface(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return fmt.Errorf("1. interface %s not found", name)
	}

	if iface.Flags&net.FlagUp == 0 {
		return fmt.Errorf("2. interface %s is not up", name)
	}

	if iface.Flags&net.FlagLoopback != 0 {
		return fmt.Errorf("3. interface %s is a loopback interface", name)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return fmt.Errorf("4. failed to get addresses for interface %s: %v", name, err)
	}

	if len(addrs) == 0 {
		return fmt.Errorf("5. interface %s has no configured addresses", name)
	}

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("6. interface %s has link-local address %s", name, ip)
		}
	}

	return nil
}
