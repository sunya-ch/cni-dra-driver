# Device Discovery

## Summary

This document outlines a mechanism for publishing a ResourceSlice of network devices. The design standardizes the representation of network devices, incorporating both common specifications and federated attributes sourced from third-party information providers.

## Motivation

Dynamic Resource Allocation (DRA) enables flexible and on-demand configuration of network devices, eliminating the need for static configuration binding to the interface name or address. Devices can be dynamically selected and configured with any supported Container Network Interface (CNI) based on user requests.

While common attributes of network-class devices—such as name, device type, and addresses—are typically well-defined and discoverable through a well-known software package like hwloc, certain CNI implementations may require additional, CNI-specific attributes. Furthermore, third-party tools can provide extended metadata, such as real-time bandwidth and latency metrics obtained from benchmarks like iPerf. To support comprehensive and flexible device discovery and selection, it is essential to define a standardized API and a mechanism that can integrate both native and external attributes.

## Design

The driver initially discovers and lists all available network devices (type=device) in the ResourceSlice, including common attributes such as interface name, IP address, and MAC address. For every network devices, the `allowMultipleAllocations` is set to `true` with a common `count` capacity set to the maximum number of pods per node by default.

In addition, the driver exposes an API socket that allows external providers to:

- Register unlisted devices (e.g., devices not of `type=device`, such as `veth` or `vlan`), or
- Add custom attributes to existing devices
- Add custom capacity to existing devices

Attributes provided by external sources are namespaced using a suffix format. For example, if a provider named `bctl` registers a `bandwidth` capacity and `rdma` attribute, the driver will add it as `bctl/bandwidth`, assuming the corresponding interface already exists.

```proto
// gRPC service definition
service DeviceRegistry {
  // Registers one or more attributes to a specific device
  rpc RegisterAttributes (RegisterAttributesRequest) returns (RegisterAttributesResponse);
}

// Request to register multiple attributes for a device
message RegisterAttributesRequest {
  // Device refers to driver identifier.
  //
  // +required
  optional string device = 1;

  // Provider refers to provider identifier (e.g., "bctl")
  //
  // +required
  optional string provider = 2;

  // Attributes defines the set of attributes for this device.
  // The name of each attribute must be unique in that set.
  //
  // The maximum number of attributes and capacities combined is 32.
  //
  // +optional
  map<string, DeviceAttribute> attributes = 2;

  // Capacity defines the set of capacities for this device.
  // The name of each capacity must be unique in that set.
  //
  // The maximum number of attributes and capacities combined is 32.
  //
  // +optional
  map<string, DeviceCapacity> capacity = 3;
}

// DeviceAttribute must have exactly one field set.
message DeviceAttribute {
  // IntValue is a number.
  //
  // +optional
  // +oneOf=ValueType
  optional int64 int = 2;

  // BoolValue is a true/false value.
  //
  // +optional
  // +oneOf=ValueType
  optional bool bool = 3;

  // StringValue is a string. Must not be longer than 64 characters.
  //
  // +optional
  // +oneOf=ValueType
  optional string string = 4;

  // VersionValue is a semantic version according to semver.org spec 2.0.0.
  // Must not be longer than 64 characters.
  //
  // +optional
  // +oneOf=ValueType
  optional string version = 5;
}

// DeviceCapacity describes a quantity associated with a device.
message DeviceCapacity {
  // Value defines how much of a certain capacity that device has.
  //
  // This field reflects the fixed total capacity and does not change.
  // The consumed amount is tracked separately by scheduler
  // and does not affect this value.
  //
  // +required
  optional .k8s.io.apimachinery.pkg.api.resource.Quantity value = 1;

  // RequestPolicy defines how this DeviceCapacity must be consumed
  // when the device is allowed to be shared by multiple allocations.
  //
  // The Device must have allowMultipleAllocations set to true in order to set a requestPolicy.
  //
  // If unset, capacity requests are unconstrained:
  // requests can consume any amount of capacity, as long as the total consumed
  // across all allocations does not exceed the device's defined capacity.
  // If request is also unset, default is the full capacity value.
  //
  // +optional
  // +featureGate=DRAConsumableCapacity
  optional CapacityRequestPolicy requestPolicy = 2;
}

// Response message
message RegisterAttributesResponse {
  bool success = 1;
  string message = 2;
}
```

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceSlice
metadata:
  name: net-slice-node1-0
spec:
  driver: net.example.com
  nodeName: node1
  pool:
    name: default-net-pool
  resourceSliceCount: 2
  devices:
    - name: eth0
      allowMultipleAllocations: true
      attributes:
        name:
          string: "eth0"
        ip:
          string: "192.168.1.10"
        mac:
          string: "00:1A:2B:3C:4D:5E"
      capacity:
        count:
          values: 200
    - name: enp1s0
      allowMultipleAllocations: true
      attributes:
        name: "enp1s0"
        ip: "192.168.1.11"
        mac: "00:1A:2B:3C:4D:5F"
        bctl/rdma:
          bool: true
      capacity:
        count:
          values: 200
        bctl/bandwidth:
          value: 4096
```

## Alternatives

The API can be a new custom resource like `CustomDevice`.

```yaml
kind: CustomDevice
spec:
  provider: bctl
  selector:
   pool.name: default-net-pool
  device: enp1s0
  addIfNotExist: false
  attributes:
    rdma:
      bool: true
  capacity:
    bandwidth:
      value: 4096
status:
  desiredNumber: 1
  appliedNumber: 1
```