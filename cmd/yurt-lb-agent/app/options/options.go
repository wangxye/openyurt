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

package options

import (
	"fmt"

	"github.com/spf13/pflag"
)

// YurtIoTDockOptions is the main settings for the yurt-iot-dock
type YurtLBAgentOptions struct {
	MetricsAddr          string
	ProbeAddr            string
	EnableLeaderElection bool
	Namespace            string
	Version              string
	Nodepool             string
	Iface                string
	Node                 string
}

func NewYurtLBAgentOptions() *YurtLBAgentOptions {
	return &YurtLBAgentOptions{
		MetricsAddr:          ":8080",
		ProbeAddr:            ":8080",
		EnableLeaderElection: false,
		Namespace:            "default",
		Version:              "",
		Nodepool:             "",
		Node:                 "",
	}
}

func ValidateOptions(options *YurtLBAgentOptions) error {
	if options.Node == "" {
		return fmt.Errorf("node name is required")
	}
	return nil
}

func (o *YurtLBAgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MetricsAddr, "metrics-bind-address", o.MetricsAddr, "The address the metric endpoint binds to.")
	fs.StringVar(&o.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&o.EnableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. "+"Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&o.Namespace, "namespace", "", "The namespace YurtLB Agent is deployed in.(just for debugging)")
	fs.StringVar(&o.Version, "version", "", "The version of edge resources deploymenet.")
	fs.StringVar(&o.Nodepool, "nodepool", "", "The nodePool YurtLB Agent is deployed in.(just for debugging)")
	fs.StringVar(&o.Iface, "iface", "", "The interface keepalived used")
	fs.StringVar(&o.Node, "node", "", "The node YurtLB Agent is deployed in.")
}
