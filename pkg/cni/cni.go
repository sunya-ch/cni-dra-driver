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

package cni

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cni-dra-driver/apis/v1alpha1"
)

type PodResourceStore interface {
	Get(podUID types.UID) []*resourcev1beta1.ResourceClaim
	Delete(podUID types.UID)
}

type UpdateStatus func(ctx context.Context, claim *resourcev1beta1.ResourceClaim, cniResult cnitypes.Result) error

type Runtime struct {
	podResourceStore PodResourceStore
	cniConfig        *libcni.CNIConfig
	driverName       string
	updateStatusFunc UpdateStatus
}

func New(
	driverName string,
	chrootDir string,
	cniPath []string,
	cniCacheDir string,
	updateStatusFunc UpdateStatus,
	podResourceStore PodResourceStore,
) *Runtime {
	exec := &chrootExec{
		Stderr:    os.Stderr,
		ChrootDir: chrootDir,
	}

	rntm := &Runtime{
		podResourceStore: podResourceStore,
		cniConfig:        libcni.NewCNIConfigWithCacheDir(cniPath, cniCacheDir, exec),
		driverName:       driverName,
		updateStatusFunc: updateStatusFunc,
	}

	return rntm
}

func (rntm *Runtime) AttachNetworks(
	ctx context.Context,
	podSandBoxID string,
	podUID string,
	podName string,
	podNamespace string,
	podNetworkNamespace string,
) error {
	claims := rntm.podResourceStore.Get(types.UID(podUID))

	klog.FromContext(ctx).Info("Runtime.AttachNetworks: attach networks on pod", "podName", podName, "podUID", podUID)

	for _, claim := range claims {
		err := rntm.handleClaim(
			ctx,
			podSandBoxID,
			podUID,
			podName,
			podNamespace,
			podNetworkNamespace,
			claim,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rntm *Runtime) handleClaim(
	ctx context.Context,
	podSandBoxID string,
	podUID string,
	podName string,
	podNamespace string,
	podNetworkNamespace string,
	claim *resourcev1beta1.ResourceClaim,
) error {
	if claim.Status.Allocation == nil ||
		len(claim.Status.Allocation.Devices.Results) != 1 ||
		len(claim.Status.Allocation.Devices.Config) != 1 ||
		claim.Status.Allocation.Devices.Results[0].Driver != rntm.driverName ||
		claim.Status.Allocation.Devices.Config[0].Opaque == nil ||
		claim.Status.Allocation.Devices.Config[0].Opaque.Driver != rntm.driverName {
		return nil
	}

	klog.FromContext(ctx).Info("Runtime.handleClaim: attach network on pod", "claim.Name", claim.Name, "podName", podName, "podUID", podUID)

	cniConfig := &v1alpha1.CNIConfig{}
	err := json.Unmarshal(claim.Status.Allocation.Devices.Config[0].Opaque.Parameters.Raw, cniConfig)
	if err != nil {
		return fmt.Errorf("Runtime.handleClaim: failed to json.Unmarshal Opaque.Parameters: %v", err)
	}

	result, err := rntm.add(
		ctx,
		podSandBoxID,
		podUID,
		podName,
		podNamespace,
		podNetworkNamespace,
		cniConfig,
	)
	if err != nil {
		return err
	}

	if rntm.updateStatusFunc != nil {
		err = rntm.updateStatusFunc(ctx, claim, result)
		if err != nil {
			return fmt.Errorf("Runtime.handleClaim: failed to update status (%v): %v", result, err)
		}
	}

	return nil
}

func (rntm *Runtime) add(
	ctx context.Context,
	podSandBoxID string,
	podUID string,
	podName string,
	podNamespace string,
	podNetworkNamespace string,
	cniConfig *v1alpha1.CNIConfig,
) (cnitypes.Result, error) {
	rt := &libcni.RuntimeConf{
		ContainerID: podSandBoxID,
		NetNS:       podNetworkNamespace,
		IfName:      cniConfig.IfName,
		Args: [][2]string{
			{"IgnoreUnknown", "true"},
			{"K8S_POD_NAMESPACE", podNamespace},
			{"K8S_POD_NAME", podName},
			{"K8S_POD_INFRA_CONTAINER_ID", podSandBoxID},
			{"K8S_POD_UID", podUID},
		},
	}

	confList, err := libcni.ConfListFromBytes(cniConfig.Config.Raw)
	if err != nil {
		return nil, fmt.Errorf("Runtime.add: failed to ConfListFromBytes: %v", err)
	}

	result, err := rntm.cniConfig.AddNetworkList(ctx, confList, rt)
	if err != nil {
		return nil, fmt.Errorf("Runtime.add: failed to AddNetwork: %v", err)
	}

	return result, nil
}

func (rntm *Runtime) DetachNetworks(
	ctx context.Context,
	podSandBoxID string,
	podUID string,
	podName string,
	podNamespace string,
	podNetworkNamespace string,
) error {
	klog.FromContext(ctx).Info("Runtime.DetachNetworks", "podSandBoxID", podSandBoxID, "podUID", podUID, "podName", podName, "podNamespace", podNamespace, "podNetworkNamespace", podNetworkNamespace)

	return nil
}
