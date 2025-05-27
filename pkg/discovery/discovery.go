package discovery

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/google/uuid"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/utils/ptr"
)

var (
	gpuPrefix    = "gpu-"
	nicPrefix    = "nic-"
	qosNicPrefix = "qos-nic-"
	capacityMap  = map[string]CapacityValue{
		gpuPrefix: CapacityValue{
			capacity: "memory",
			value:    resource.MustParse("80Gi"),
		},
		nicPrefix: CapacityValue{
			capacity: "bandwidth",
			value:    resource.MustParse("10Gi"),
		},
		qosNicPrefix: CapacityValue{
			capacity: "bandwidth",
			value:    resource.MustParse("10Gi"),
		},
	}
	modelMap = map[string]string{
		gpuPrefix:    "LATEST-GPU-MODEL",
		nicPrefix:    "LATEST-NIC-MODEL",
		qosNicPrefix: "LATEST-QOS-NIC-MODEL",
	}
	one = resource.MustParse("1")
)

type AllocatableDevices map[string]resourceapi.Device

type Config struct {
	NumDevices                     int
	NumSharedDevices               int
	NumSharedDevicesWithConsumable int
}

type DeviceDiscovery struct {
	allocatable AllocatableDevices
}

type CapacityValue struct {
	capacity string
	value    resource.Quantity
}

func NewDeviceDiscovery(config Config) (*DeviceDiscovery, error) {
	allocatable, err := enumerateAllPossibleDevices(config.NumDevices, config.NumSharedDevices, config.NumSharedDevicesWithConsumable)
	if err != nil {
		return nil, fmt.Errorf("error enumerating all possible devices: %v", err)
	}
	return &DeviceDiscovery{
		allocatable: allocatable,
	}, nil
}

func (m *DeviceDiscovery) GetResources(poolName string) resourceslice.DriverResources {
	var devices []resourceapi.Device
	for _, device := range m.allocatable {
		devices = append(devices, device)
	}
	var resources resourceslice.DriverResources
	resources.Pools = map[string]resourceslice.Pool{
		poolName: resourceslice.Pool{
			Slices: []resourceslice.Slice{{
				Devices: devices,
			},
			},
		},
	}
	return resources
}

func (m *DeviceDiscovery) IsAllocatable(deviceName string) bool {
	_, found := m.allocatable[deviceName]
	return found
}

func enumerateAllPossibleDevices(numGPUs, numShared, numSharedWithConsumable int) (AllocatableDevices, error) {
	seed := os.Getenv("NODE_NAME")
	gpuUuids := generateUUIDs(gpuPrefix, seed, numGPUs)
	nicUuid := generateUUIDs(nicPrefix, seed, numShared)
	qosNicUuid := generateUUIDs(qosNicPrefix, seed, numShared)

	alldevices := make(AllocatableDevices)
	for i, uuid := range gpuUuids {
		device := generateDevice(gpuPrefix, i, uuid, false, false)
		alldevices[device.Name] = device
	}
	for i, uuid := range nicUuid {
		device := generateDevice(nicPrefix, i, uuid, true, false)
		alldevices[device.Name] = device
	}
	for i, uuid := range qosNicUuid {
		device := generateDevice(qosNicPrefix, i, uuid, true, true)
		alldevices[device.Name] = device
	}
	return alldevices, nil
}

func generateDevice(prefix string, i int, uuid string, shared bool, consumable bool) resourceapi.Device {
	capacityValue := capacityMap[prefix]
	deviceCapacity := resourceapi.DeviceCapacity{Value: capacityValue.value}
	if consumable {
		deviceCapacity.ClaimPolicy = &resourceapi.CapacityClaimPolicy{
			Range: &resourceapi.CapacityClaimPolicyRange{
				Minimum: one,
			},
		}
	}
	model := modelMap[prefix]
	device := resourceapi.Device{
		Name: fmt.Sprintf("%s%d", prefix, i),
		Basic: &resourceapi.BasicDevice{
			Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
				"index": {
					IntValue: ptr.To(int64(i)),
				},
				"uuid": {
					StringValue: ptr.To(uuid),
				},
				"model": {
					StringValue: ptr.To(model),
				},
				"driverVersion": {
					VersionValue: ptr.To("1.0.0"),
				},
			},
			Capacity: map[resourceapi.QualifiedName]resourceapi.DeviceCapacity{
				resourceapi.QualifiedName(capacityValue.capacity): deviceCapacity,
			},
		},
	}
	if shared {
		device.Basic.Shared = ptr.To(true)
	}
	return device
}

func generateUUIDs(prefix, seed string, count int) []string {
	rand := rand.New(rand.NewSource(hash(seed)))

	uuids := make([]string, count)
	for i := 0; i < count; i++ {
		charset := make([]byte, 16)
		rand.Read(charset)
		uuid, _ := uuid.FromBytes(charset)
		uuids[i] = prefix + uuid.String()
	}

	return uuids
}

func hash(s string) int64 {
	h := int64(0)
	for _, c := range s {
		h = 31*h + int64(c)
	}
	return h
}
