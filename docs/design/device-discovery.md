# Device Discovery

The driver discovers and initially lists up all available network device `type=device` in the ResourceSlice with common attributes such as interface name, and IP/Mac addresses.

Additionally, it provides a API socket to receive registration of new device or new attribute to the ResourceSlice from external providers. The attribute from external provider will be suffix. For example, provider `bctl` registers attribute `bandwidth` to the driver. The attribute `bctl.bandwidth` will be added if the interface name exists.

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