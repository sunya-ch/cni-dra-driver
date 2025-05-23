/*
Copyright 2025 The Kubernetes Authors.

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

package dra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/klog/v2"
)

var _ kubeletplugin.DRAPlugin = &Driver{}

type PodResourceStore interface {
	Add(podUID types.UID, allocation *resourcev1beta1.ResourceClaim)
}

type Driver struct {
	driverName       string
	kubeClient       kubernetes.Interface
	draPlugin        *kubeletplugin.Helper
	podResourceStore PodResourceStore

	prepareResourcesFailure   error
	failPrepareResourcesMutex sync.Mutex

	unprepareResourcesFailure   error
	failUnprepareResourcesMutex sync.Mutex
}

func Start(
	ctx context.Context,
	driverName string,
	nodeName string,
	kubeClient kubernetes.Interface,
	podResourceStore PodResourceStore,
) (*Driver, error) {
	d := &Driver{
		driverName:       driverName,
		kubeClient:       kubeClient,
		podResourceStore: podResourceStore,
	}

	driverPluginPath := filepath.Join("/var/lib/kubelet/plugins/", driverName)

	err := os.MkdirAll(driverPluginPath, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin path %s: %v", driverPluginPath, err)
	}

	plugin, err := kubeletplugin.Start(
		ctx,
		d,
		kubeletplugin.KubeClient(kubeClient),
		kubeletplugin.NodeName(nodeName),
		kubeletplugin.DriverName(driverName),
	)

	if err != nil {
		return nil, fmt.Errorf("start kubelet plugin: %w", err)
	}
	d.draPlugin = plugin

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(context.Context) (bool, error) {
		status := d.draPlugin.RegistrationStatus()
		if status == nil {
			return false, nil
		}
		return status.PluginRegistered, nil
	})
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Driver) Stop() {
	if d.draPlugin != nil {
		d.draPlugin.Stop()
	}
}

func (d *Driver) nodePrepareResource(ctx context.Context, claim *resourcev1beta1.ResourceClaim) ([]kubeletplugin.Device, error) {
	if claim == nil {
		return nil, fmt.Errorf("nil claim")
	}
	if claim.Status.Allocation == nil {
		return nil, fmt.Errorf("claim %s/%s not allocated", claim.Namespace, claim.Name)
	}

	if claim.UID != types.UID(claim.UID) {
		return nil, fmt.Errorf("claim %s/%s got replaced", claim.Namespace, claim.Name)
	}

	for _, reserved := range claim.Status.ReservedFor {
		if reserved.Resource != "pods" || reserved.APIGroup != "" {
			klog.FromContext(ctx).Info("claim reference unsupported", "reserved", reserved)

			continue
		}

		klog.FromContext(ctx).Info("nodePrepareResource: Claim Request reserved for pod", "claimReq.UID", claim.UID, "reserved.Name", reserved.Name, "reserved.UID", reserved.UID)
		d.podResourceStore.Add(reserved.UID, claim)
	}

	var devices []kubeletplugin.Device
	for _, result := range claim.Status.Allocation.Devices.Results {
		device := kubeletplugin.Device{
			PoolName:   result.Pool,
			DeviceName: result.Device,
		}
		devices = append(devices, device)
	}

	klog.FromContext(ctx).Info("nodePrepareResource: Devices for Claim", "claim.UID", claim.UID, "devices", devices)

	return devices, nil
}

func (d *Driver) nodeUnprepareResource(_ context.Context, _ string) error {
	// TODO
	return nil
}

// modified

// PrepareResourceClaims
func (d *Driver) PrepareResourceClaims(ctx context.Context, claims []*resourcev1beta1.ResourceClaim) (result map[types.UID]kubeletplugin.PrepareResult, err error) {
	if failure := d.getPrepareResourcesFailure(); failure != nil {
		return nil, failure
	}
	result = make(map[types.UID]kubeletplugin.PrepareResult)
	for _, claim := range claims {

		devices, err := d.nodePrepareResource(ctx, claim)
		var claimResult kubeletplugin.PrepareResult
		if err != nil {
			klog.FromContext(ctx).Error(err, "error unpreparing ressources for a claim", "claim.Namespace", claim.Namespace, "claim.Name", claim.Name)
			claimResult.Err = err
		} else {
			claimResult.Devices = devices
		}
		result[claim.UID] = claimResult
	}
	return result, nil
}

func (d *Driver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (result map[types.UID]error, err error) {
	result = make(map[types.UID]error)

	if failure := d.getUnprepareResourcesFailure(); failure != nil {
		return nil, failure
	}

	for _, claimRef := range claims {
		uid := string(claimRef.UID)
		err := d.nodeUnprepareResource(ctx, uid)
		result[claimRef.UID] = err
	}
	return result, nil
}

func (d *Driver) getPrepareResourcesFailure() error {
	d.failPrepareResourcesMutex.Lock()
	defer d.failPrepareResourcesMutex.Unlock()
	return d.prepareResourcesFailure
}

func (d *Driver) getUnprepareResourcesFailure() error {
	d.failUnprepareResourcesMutex.Lock()
	defer d.failUnprepareResourcesMutex.Unlock()
	return d.unprepareResourcesFailure
}
