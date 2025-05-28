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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/nri/pkg/stub"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cni-dra-driver/pkg/cni"
	"sigs.k8s.io/cni-dra-driver/pkg/discovery"
	"sigs.k8s.io/cni-dra-driver/pkg/dra"
	"sigs.k8s.io/cni-dra-driver/pkg/nri"
	"sigs.k8s.io/cni-dra-driver/pkg/status"
	"sigs.k8s.io/cni-dra-driver/pkg/store"
)

type runOptions struct {
	pluginName    string
	pluginIndex   string
	CNIPath       string
	CNICacheDir   string
	ChrootDir     string
	DRADriverName string
	NodeName      string

	numDevices                     int
	numSharedDevices               int
	numSharedDevicesWithConsumable int
}

func newCmdRun() *cobra.Command {
	runOpts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the cni-dra-driver",
		Long:  `Run the cni-dra-driver`,
		Run: func(cmd *cobra.Command, args []string) {
			runOpts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(
		&runOpts.pluginName,
		"plugin-name",
		"cni-dra-driver",
		"Plugin name to register to NRI.",
	)

	cmd.Flags().StringVar(
		&runOpts.pluginIndex,
		"plugin-index",
		"",
		"plugin index to register to NRI.",
	)

	cmd.Flags().StringVar(
		&runOpts.CNIPath,
		"cni-path",
		"/opt/cni/bin",
		"CNI Path.",
	)

	cmd.Flags().StringVar(
		&runOpts.CNICacheDir,
		"cni-cache-dir",
		"/var/lib/cni/cni-dra-driver",
		"CNI Cache dir.",
	)

	cmd.Flags().StringVar(
		&runOpts.ChrootDir,
		"chroot-dir",
		"/hostroot",
		"ChrootDir.",
	)

	cmd.Flags().StringVar(
		&runOpts.DRADriverName,
		"dra-driver-name",
		"cni.dra.networking.x-k8s.io",
		"DRA Driver Name.",
	)

	cmd.Flags().StringVar(
		&runOpts.NodeName,
		"node-name",
		"",
		"Node Name.",
	)

	cmd.Flags().IntVar(
		&runOpts.numDevices,
		"num-devices",
		8,
		"The number of devices to be generated.",
	)

	cmd.Flags().IntVar(
		&runOpts.numSharedDevices,
		"shared-devices",
		1,
		"The number of shared devices without consumable capacity to be generated.",
	)
	cmd.Flags().IntVar(
		&runOpts.numSharedDevicesWithConsumable,
		"shared-devices-with-consumable-capacity",
		1,
		"The number of shared devices with consumable capacity to be generated.",
	)

	return cmd
}

func (ro *runOptions) run(ctx context.Context) {
	opts := []stub.Option{
		stub.WithPluginName(ro.pluginName),
		stub.WithPluginIdx(ro.pluginIndex),
	}

	clientCfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to InClusterConfig: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to NewForConfig: %v\n", err)
		os.Exit(1)
	}

	memoryStore := store.NewMemory()
	config := discovery.Config{
		NumDevices:                     ro.numDevices,
		NumSharedDevices:               ro.numSharedDevices,
		NumSharedDevicesWithConsumable: ro.numSharedDevicesWithConsumable,
	}
	deviceDiscovery, err := discovery.NewDeviceDiscovery(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initiate device discovery: %v\n", err)
		os.Exit(1)
	}

	cnish := status.CNIStatusHandler{
		ClientSet: clientset,
	}

	cni := cni.New(
		ro.DRADriverName,
		ro.ChrootDir,
		[]string{ro.CNIPath},
		ro.CNICacheDir,
		cnish.UpdateStatus,
		memoryStore,
	)

	draDriver, err := dra.Start(
		ctx,
		ro.DRADriverName,
		ro.NodeName,
		clientset,
		memoryStore,
		*deviceDiscovery,
		cni,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dra.Start: %v\n", err)
		os.Exit(1)
	}
	defer draDriver.Stop()

	p := &nri.Plugin{
		ClientSet: clientset,
		CNI:       cni,
	}

	p.Stub, err = stub.New(p, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create plugin stub: %v\n", err)
		os.Exit(1)
	}

	err = p.Stub.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plugin exited with error: %v\n", err)
		os.Exit(1)
	}
}
