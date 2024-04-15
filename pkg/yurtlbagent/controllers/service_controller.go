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

package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openyurtio/openyurt/cmd/yurt-lb-agent/app/options"
	network "github.com/openyurtio/openyurt/pkg/apis/network"
	netv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/network/v1alpha1"
	"github.com/openyurtio/openyurt/pkg/yurtlbagent/util"
)

const (
	VRIDLabelKey      = "service.openyurt.io/vrid"
	BACKUPState       = "BACKUP"
	LOADBALANCERCLASS = "service.openyurt.io/viplb"
)

var controllerResource = netv1alpha1.SchemeGroupVersion.WithResource("poolservices")

// TODO: poolendpoints check

func Format(format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s: %s", "yurt-lb-agent: controller PoolServiceReconciler", s)
}

type PoolServiceReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Nodepool          string
	Node              string
	Namespace         string
	Keepalived        *util.Keepalived
	LoadBalancerClass string
	Services          map[string][]net.IP
}

// +kubebuilder:rbac:groups=network.openyurt.io,resources=poolservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.openyurt.io,resources=poolservices/status,verbs=get;update;patch
// TODO: poolendpoints

func (r *PoolServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof(Format("reconcile PoolService %s/%s", req.Namespace, req.Name))

	// Fetch the poolservice
	poolservice := &netv1alpha1.PoolService{}
	if err := r.Get(ctx, req.NamespacedName, poolservice); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		klog.Errorf(Format("get PoolService %s/%s error %v", req.Namespace, req.Name, err))
		return ctrl.Result{}, err
	}
	klog.Infof(Format("get PoolService  %s/%s", req.Namespace, req.Name))

	if poolservice.DeletionTimestamp != nil {
		return r.reconcileDelete(ctx, poolservice)
	}

	return r.reconcileNormal(ctx, poolservice)
}

func (r *PoolServiceReconciler) reconcileDelete(_ context.Context, poolservice *netv1alpha1.PoolService) (ctrl.Result, error) {
	klog.Infof(Format("reconcileDelete poolservice %s/%s", poolservice.Namespace, poolservice.Name))

	return r.deleteBalancer(poolservice)
}

func (r *PoolServiceReconciler) reconcileNormal(ctx context.Context, poolservice *netv1alpha1.PoolService) (ctrl.Result, error) {
	klog.Infof(Format("reconcileNormal poolservice %s/%s", poolservice.Namespace, poolservice.Name))
	podExistsOnNode, err := r.filterByPod(ctx, poolservice)
	if err != nil || !podExistsOnNode {
		return r.deleteBalancer(poolservice)
	}

	if len(poolservice.Status.LoadBalancer.Ingress) == 0 {
		return r.deleteBalancer(poolservice)
	}

	vrid, err := getVrid(poolservice)
	if err != nil {
		klog.Errorf(Format("poolservice %s/%s has invalid vrid: %v", poolservice.Namespace, poolservice.Name, err))
		return r.deleteBalancer(poolservice)
	}

	lbIPs := []net.IP{}
	for _, ingress := range poolservice.Status.LoadBalancer.Ingress {
		lbIP := net.ParseIP(ingress.IP)
		if lbIP == nil {
			klog.Errorf(Format("Unable to parse lb IP %s for poolservice %s/%s", ingress.IP, poolservice.Namespace, poolservice.Name))
			return r.deleteBalancer(poolservice)
		}
		lbIPs = append(lbIPs, lbIP)
	}

	name := types.NamespacedName{Namespace: poolservice.Namespace, Name: poolservice.Name}.String()
	_, ok := r.Services[name]
	if ok && compareIPs(lbIPs, r.Services[name]) && vrid == r.Keepalived.LBServices[name].Vrid {
		klog.Infof("poolservice %s LBIPs and vrid not change", poolservice.Namespace, poolservice.Name)
		return ctrl.Result{}, nil
	}

	var ips []string
	for _, ip := range lbIPs {
		ips = append(ips, ip.String())
	}

	svc := r.Keepalived.LBServices[name]
	if !ok {
		svc = util.LBService{Name: name, Priority: getPriority()}
	}
	svc.Vrid = vrid
	svc.Ips = ips
	r.Keepalived.LBServices[name] = svc

	if err = r.Keepalived.LoadConfig(); err != nil {
		return ctrl.Result{}, fmt.Errorf("loading keepalived configuration error: %v", err)
	}

	if err = r.Keepalived.ReloadKeepalived(); err != nil {
		return ctrl.Result{}, fmt.Errorf("reloading keepalived configuration error: %v", err)
	}
	r.Services[name] = lbIPs

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PoolServiceReconciler) SetupWithManager(mgr ctrl.Manager, opts *options.YurtLBAgentOptions) error {
	if _, err := mgr.GetRESTMapper().KindFor(controllerResource); err != nil {
		klog.Infof("resource %s doesn't exist", controllerResource.String())
		return err
	}

	r.Nodepool = opts.Nodepool
	r.Node = opts.Node
	r.Namespace = opts.Namespace
	r.LoadBalancerClass = LOADBALANCERCLASS
	r.Services = make(map[string][]net.IP)

	keepalivedConf := &util.Keepalived{
		State:      BACKUPState,
		Iface:      opts.Iface,
		LBServices: make(map[string]util.LBService),
	}
	var err error
	err = keepalivedConf.LoadTemplate()
	if err != nil {
		return fmt.Errorf("load keepalived template err, %v", err)
	}
	err = keepalivedConf.LoadConfig()
	if err != nil {
		return fmt.Errorf("write keepalived config err, %v", err)
	}
	r.Keepalived = keepalivedConf
	go func() {
		// TODO: retry if error
		err = keepalivedConf.Start()
		klog.Errorf(Format("start keepalived err, %v", err))
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.PoolService{}).
		WithEventFilter(NewPoolServicePredicated(r.Nodepool)).
		Complete(r)
}

func (r *PoolServiceReconciler) deleteBalancer(poolservice *netv1alpha1.PoolService) (ctrl.Result, error) {
	name := types.NamespacedName{Namespace: poolservice.Namespace, Name: poolservice.Name}.String()
	if _, ok := r.Keepalived.LBServices[name]; !ok {
		klog.Infof(Format("poolservice %s/%s already deleted", poolservice.Namespace, poolservice.Name))
		return ctrl.Result{}, nil
	}
	delete(r.Keepalived.LBServices, name)
	err := r.Keepalived.LoadConfig()
	if err != nil {
		klog.Errorf(Format("error loading keepalived configuration: %v", err))
	}
	err = r.Keepalived.ReloadKeepalived()
	if err != nil {
		klog.Errorf(Format("update keepalived configuration error: %v", err))
	}

	delete(r.Services, name)
	klog.Infof(Format("poolservice %s/%s successfully deleted", poolservice.Namespace, poolservice.Name))

	return ctrl.Result{}, nil
}

func (r *PoolServiceReconciler) filterByPod(ctx context.Context, poolservice *netv1alpha1.PoolService) (bool, error) {
	// get service by poolservice
	service := &v1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: poolservice.Namespace, Name: poolservice.Labels[network.LabelServiceName]}, service); err != nil {
		klog.Errorf(Format("get Service %s/%s error %v", poolservice.Namespace, poolservice.Labels[network.LabelServiceName], err))
		return false, err
	}
	klog.Infof(Format("get Service %s/%s", poolservice.Namespace, poolservice.Name))

	podList := &v1.PodList{}
	err := r.List(ctx, podList, client.InNamespace(service.Namespace), client.MatchingLabels(service.Spec.Selector))
	if err != nil {
		klog.Errorf(Format("get pod list for service %s/%s error %v", service.Namespace, service.Name, err))
		return false, err
	}
	klog.Infof(Format("get %v pod items for service %s/%s", len(podList.Items), service.Namespace, service.Name))

	podExistsOnNode := false
	for _, pod := range podList.Items {
		if pod.Spec.NodeName == r.Node {
			podExistsOnNode = true
			break
		}
	}

	return podExistsOnNode, nil
}

func compareIPs(ips1 []net.IP, ips2 []net.IP) bool {
	if len(ips1) != len(ips2) {
		return false
	}
	for _, ip1 := range ips1 {
		flag := false
		for _, ip2 := range ips2 {
			if ip1.Equal(ip2) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}
	return true
}

func getPriority() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(200)
}

func getVrid(poolservice *netv1alpha1.PoolService) (int, error) {
	vrid, err := strconv.Atoi(poolservice.Labels[VRIDLabelKey])
	if err != nil {
		return -1, err
	}
	if vrid < 0 || vrid > 255 {
		return -1, fmt.Errorf("invalid vrid %v", vrid)
	}
	return vrid, nil
}
