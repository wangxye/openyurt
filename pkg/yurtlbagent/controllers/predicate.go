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

package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	network "github.com/openyurtio/openyurt/pkg/apis/network"
	"github.com/openyurtio/openyurt/pkg/apis/network/v1alpha1"
	netv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/network/v1alpha1"
)

func NewPoolServicePredicated(poolName string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			ps, ok := createEvent.Object.(*v1alpha1.PoolService)
			if !ok {
				return false
			}
			return predicatedPoolService(ps, poolName)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldPs, ok := updateEvent.ObjectOld.(*v1alpha1.PoolService)
			if !ok {
				return false
			}

			newPS, ok := updateEvent.ObjectNew.(*v1alpha1.PoolService)
			if !ok {
				return false
			}
			return predicatedPoolService(oldPs, poolName) ||
				predicatedPoolService(newPS, poolName)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			ps, ok := deleteEvent.Object.(*v1alpha1.PoolService)
			if !ok {
				return false
			}
			return predicatedPoolService(ps, poolName)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			ps, ok := genericEvent.Object.(*v1alpha1.PoolService)
			if !ok {
				return false
			}
			return predicatedPoolService(ps, poolName)
		},
	}
}

func predicatedPoolService(ps *v1alpha1.PoolService, poolName string) bool {
	return matchLoadBalancerClass(ps, LOADBALANCERCLASS) && matchNodePool(ps, poolName)
}

func matchLoadBalancerClass(poolservice *netv1alpha1.PoolService, loadBalancerClass string) bool {
	if poolservice.Spec.LoadBalancerClass == nil {
		return false
	}
	if *poolservice.Spec.LoadBalancerClass != loadBalancerClass {
		return false
	}
	return true
}

func matchNodePool(poolservice *netv1alpha1.PoolService, nodepool string) bool {
	if poolservice.Labels == nil {
		return false
	}

	if poolservice.Labels[network.LabelNodePoolName] != nodepool {
		return false
	}
	return true
}
