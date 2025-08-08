# Device Discovery

## Summary

This document outlines a mechanism for publishing a ResourceSlice of network devices. The design standardizes the representation of network devices, incorporating both common specifications and federated attributes sourced from third-party information providers.

## Motivation

Dynamic Resource Allocation (DRA) enables flexible and on-demand configuration of network devices, eliminating the need for static configuration binding to the interface name or address. Devices can be dynamically selected and configured with any supported Container Network Interface (CNI) based on user requests.

While common attributes of network-class devices—such as name, device type, and addresses—are typically well-defined and discoverable through a well-known software package like hwloc, certain CNI implementations may require additional, CNI-specific attributes. Furthermore, third-party tools can provide extended metadata, such as real-time bandwidth and latency metrics obtained from benchmarks like iPerf. To support comprehensive and flexible device discovery and selection, it is essential to define a standardized API and a mechanism that can integrate both native and external attributes.

## Design

The driver initially discovers and lists all available network devices (type=device) in the ResourceSlice, including common attributes such as interface name, IP address, and MAC address.

In addition, the driver exposes an API socket that allows external providers to:

- Register unlisted devices (e.g., devices not of `type=device`, such as `veth` or `vlan`), or
- Add custom attributes to existing devices in the `ResourceSlice`.

Attributes provided by external sources are namespaced using a suffix format. For example, if a provider named `bctl` registers a `bandwidth` attribute, the driver will add it as `bctl.bandwidth`, assuming the corresponding interface already exists.

```proto
// gRPC service definition
service DeviceRegistry {
  // Registers one or more attributes to a specific device
  rpc RegisterAttributes (RegisterAttributesRequest) returns (RegisterAttributesResponse);
}

// Request to register multiple attributes for a device
message RegisterAttributesRequest {
  // Device name (e.g., "eth1")
  string device_name = 1;

  // Provider name (e.g., "bctl")
  string provider = 2;

  // List of attributes to register
  repeated Attribute attributes = 3;
}

// Single attribute entry (name/value)
message Attribute {
  string name = 1;   // e.g., "bandwidth"
  string value = 2;  // e.g., "1G"
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
      attributes:
        name: "eth0"
        ip: "192.168.1.10"
        mac: "00:1A:2B:3C:4D:5E"
    - name: eth1
      attributes:
        name: "eth1"
        ip: "192.168.1.11"
        mac: "00:1A:2B:3C:4D:5F"
        bctl.bandwidth: "1Gbps"
```

## Alternatives

The API can be a new custom resource like `CustomAttribute`.

```yaml
kind: CustomAttributes
spec:
  provider: bctl
  selector:
   pool.name: default-net-pool
  deviceName: eth0
  attributes:
  - name: bandwidth
    value: 1G
status:
  desiredNumber: 1
  appliedNumber: 1
```