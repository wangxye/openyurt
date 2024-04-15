/*
Copyright 2024 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the License);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an AS IS BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package viploadbalancer

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	network "github.com/openyurtio/openyurt/pkg/apis/network"
	netv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/network/v1alpha1"
)

// newVipAgentDaemon initialize the configuration of yurt-lb-agent
func newVipAgentDaemon(poolService netv1alpha1.PoolService) (*appsv1.DaemonSetSpec, error) {

	// Otherwise, the default configuration is used to start
	ver, ns, err := DefaultVersion(poolService)
	if err != nil {
		return nil, err
	}
	poolName := poolService.Labels[network.LabelNodePoolName]
	vipAgent := &appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app":           vipAgentName(poolName),
				"control-plane": VipAgentControlPlane,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":           vipAgentName(poolName),
					"control-plane": VipAgentControlPlane,
				},
				Namespace: ns,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            VipAgentName,
						Image:           fmt.Sprintf("%s:%s", VipAgentImage, ver),
						ImagePullPolicy: corev1.PullAlways,
						Args: []string{
							"--health-probe-bind-address=:8081",
							"--metrics-bind-address=127.0.0.1:8080",
							"--leader-elect=false",
							"--iface=eth0",
							"--node=$(NODE_NAME)",
							fmt.Sprintf("--namespace=%s", ns),
							fmt.Sprintf("--nodepool=%s", poolName),
						},
						LivenessProbe: &corev1.Probe{
							InitialDelaySeconds: 15,
							PeriodSeconds:       20,
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt(8081),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromInt(8081),
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("512m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1024m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: pointer.Bool(false),
						},
						Env: []corev1.EnvVar{
							{
								Name: "NODE_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
						},
					},
				},
				TerminationGracePeriodSeconds: pointer.Int64(10),
				SecurityContext: &corev1.PodSecurityContext{
					RunAsUser: pointer.Int64(65532),
				},
				HostNetwork: true,
			},
		},
	}

	return vipAgent, nil
}

func vipAgentName(nodepool string) string {
	return fmt.Sprintf("%s-%s", VipAgentName, nodepool)
}
