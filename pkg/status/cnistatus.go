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

package status

import (
	"context"
	"encoding/json"
	"fmt"

	cnitypes "github.com/containernetworking/cni/pkg/types"
	cni100 "github.com/containernetworking/cni/pkg/types/100"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
)

type CNIStatusHandler struct {
	ClientSet clientset.Interface
}

func (cnish *CNIStatusHandler) UpdateStatus(ctx context.Context, claim *resourcev1beta1.ResourceClaim, result cnitypes.Result) error {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("cni.handleClaim: failed to json.Marshal result (%v): %v", result, err)
	}

	cniResult, err := cni100.NewResultFromResult(result)
	if err != nil {
		return fmt.Errorf("cni.handleClaim: failed to NewResultFromResult result (%v): %v", result, err)
	}

	data := runtime.RawExtension{
		Raw: resultBytes,
	}
	claim.Status.Devices = append(claim.Status.Devices, resourcev1beta1.AllocatedDeviceStatus{
		Driver:      claim.Status.Allocation.Devices.Results[0].Driver,
		Pool:        claim.Status.Allocation.Devices.Results[0].Pool,
		Device:      claim.Status.Allocation.Devices.Results[0].Device,
		Data:        &data,
		NetworkData: cniResultToNetworkData(cniResult),
		ShareUID:    claim.Status.Allocation.Devices.Results[0].ShareUID,
	})

	_, err = cnish.ClientSet.ResourceV1beta1().ResourceClaims(claim.GetNamespace()).UpdateStatus(ctx, claim, v1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cni.handleClaim: failed to update resource claim status (%v): %v", result, err)
	}

	return nil
}

func cniResultToNetworkData(cniResult *cni100.Result) *resourcev1beta1.NetworkDeviceData {
	networkData := &resourcev1beta1.NetworkDeviceData{}

	for _, ip := range cniResult.IPs {
		networkData.IPs = append(networkData.IPs, ip.Address.String())
	}

	for _, ifs := range cniResult.Interfaces {
		// Only pod interfaces can have sandbox information
		if ifs.Sandbox != "" {
			networkData.InterfaceName = ifs.Name
			networkData.HardwareAddress = ifs.Mac
		}
	}

	return networkData
}
