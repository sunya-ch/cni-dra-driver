# Device Configuration API

## Summary

This document defines the API and mechanism to configure and manage CNI network interfaces in Kubernetes using the CNI DRA Driver `cni.dra.networking.x-k8s.io`. In addition, it also outlines how the status of these network interfaces (allocated devices) is reported.

The design aims to support all possible functionalities the Container Network Interface (CNI) project offers while ensuring compliance with its specifications.

## Motivation

Configuring and reporting network interfaces in Kubernetes currently lacks a standardized and native approach. Existing solutions such as [Multus](https://github.com/k8snetworkplumbingwg/multus-cni) address this challenge by relying on CNI, custom resources and pod annotations. However, these solutions do not provide a fully integrated and standardized method within Kubernetes. The Device Configuration API of the CNI-DRA-Driver aims to fill this gap by leveraging the CNI API together with the DRA API to provide a modern and more Kubernetes integrated mechanism to configure and report network interfaces on pods.

This document defines the configuration API for configuring network interfaces on pods using CNI and outlines the behaviors and interactions during the operations (e.g. `ADD` and `DEL`) on the network interfaces and pods. The capabilities and limitations of this approach are also highlighted to ensure a clear understanding of its scope.

The configuration API must be extensible to support runtime variables derived from dynamic allocation, such as the interface name or host name, along with their corresponding values — for example, a pre-configured CIDR range.

Additionally, this solution will serve as a reference implementation for the [Multi-Network](https://github.com/kubernetes-sigs/multi-network) project and for the [KEP-4817 (Resource Claim Status With Possible Standardized Network Interface Data)](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/4817-resource-claim-device-status/README.md).

## Design

A new API with the Kind `CNIConfig` will be introduced under the `cni.networking.x-k8s.io` Group. This API will be providing the CNI configuration along with necessary parameters and optional fields for invoking CNI plugins. The configuration data specified in the `opaque.parameters` will be reflected in the ResourceClaim status by Kubernetes. This reported status will provide the necessary details for executing CNI operations such as `ADD` on pod creation and `DEL` on pod deletion which will enable seamless lifecycle management.

Each ResourceClaim can be claimed by at most one pod as the ResourceClaim status represents network interfaces configured specifically for that pod. Since the devices configured via CNI are pod scoped rather than container scoped, the ResourceClaims must be associated at the pod level.

As each ResourceClaim can be claimed by at most one pod, a ResourceClaimTemplate can be used to link the same ResourceClaim configuration to multiple pods.

To support scenarios where multiple network interfaces are required, a ResourceClaim can contain one or multiple requests and a pod can reference one or multiple ResourceClaims.

### Configuration

This API defines the parameters and CNI configuration required to invoke CNI plugins. The `CNIConfig` object encapsulates two fields:
* `CNIVersion`: specifies 
* `IfName`: specifies the name of the network interface to be configured.
* `Plugins`: contains list of plugins.

The CNI configuration represented as a generic type `runtime.RawExtension`.

```golang
// CNIConfig is the object used in ResourceClaim.specs.devices.config opaque parameters.
type CNIConfig struct {
  metav1.TypeMeta `json:",inline"`

  // CNIVersion defines CNI version.
  CNIVersion Version `json:"cniVersion"`

  // IfName defines network interface to be configured.
  IfName string `json:"name"`

  // Plugins contain list of CNI plugin definitions.
  Plugins []CNIPlugin `json:"plugins"`
}

// CNIPlugin defines a CNI plugin definition.
type CNIPlugin struct {
  // Type defines valid type of plugin (available in cni-bin path)
  Type string `json:"type"`

  // Config represents the static CNI Config.
  Config runtime.RawExtension `json:"config"`

  // Args defines CNI-specific dynamic arguments
  Args []CNIArg
}

// CNIArg defines an argument to set value in CNI config in runtime.
type CNIArg struct {
  // fieldPath defines field path of the argument to set
  fieldPath string `json:"field"`

  // Value defines a constant value
  Value string `json:"value"`

  // Source for the CNI argument's value. Cannot be used if value is not empty.
  // +optional
  ValueFrom *CNIArgSource `json:"valueFrom,omitempty" protobuf:"bytes,3,opt,name=valueFrom"`
}

// CNIArgSource defines a source of argument value.
type CNIArgSource struct {
  // Attribute uses device attribute as source
  // +oneOf
  // +optional
  AttributeRef *string `json:"attribute"`

  // ResourceSliceFieldRef selects a field of ResourceSlice where the selected device belong to: spec.nodeName
  // +oneOf
  // +optional
  ResourceSliceFieldRef *ObjectFieldSelector `json:"resourceSliceFieldRef,omitempty" protobuf:"bytes,1,opt,name=resourceSliceFieldRef"`

  // Selects a key of a ConfigMap.
  // For example, define a range of CIDR based on nodeName and interface name
  //
  // +optional
  ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty" protobuf:"bytes,3,opt,name=configMapKeyRef"`
}

type ConfigMapKeySelector struct {
  // The ConfigMap to select from.
  LocalObjectReference `json:",inline" protobuf:"bytes,1,opt,name=localObjectReference"`

  // The key to select.
  // +oneOf
  // +optional
  Key string `json:"key" protobuf:"bytes,2,opt,name=key"`

  // AttributeKey uses device attribute as a config map key such as name of interface
  // +oneOf
  // +optional
  AttributeKey *string `json:"attribute"`

  // ResourceSliceFieldKey uses a field of ResourceSlice as a config map key such as nodeName
  // +oneOf
  // +optional
  ResourceSliceFieldKey *ObjectFieldSelector `json:"resourceSliceFieldRef,omitempty" protobuf:"bytes,1,opt,name=resourceSliceFieldRef"`
}


```

Requests using the device class `cni.networking.x-k8s.io` must include exactly one configuration attached to it, so each configuration must point to a single request (one-to-one relationship between the config (CNI object) and the request). This configuration must specify the driver name `cni.dra.networking.x-k8s.io` and the corresponding `CNIConfig` object in the `opaque.parameters` field. 

Each request will configure one network interface in the pod.

Here is an example below of a ResourceClaim definition that includes the CNI config and parameters:
```yaml
apiVersion: resource.k8s.io/v1beta1
kind: ResourceClaim
metadata:
  name: one-macvlan-attachment
spec:
  devices:
    requests:
    - name: macvlan0
      deviceClassName: cni.networking.x-k8s.io
      allocationMode: ExactCount
      count: 1
    config:
    - requests:
      - macvlan0
      opaque:
        driver: cni.dra.networking.x-k8s.io
        parameters: # CNIParameters with the GVK, interface name and CNI Config (in YAML format).
          apiVersion: cni.networking.x-k8s.io/v1alpha1
          kind: CNI
          ifName: "net1"
          cniVersion: 1.0.0
          plugins:
          - type: macvlan
            config:
              mode: bridge
              ipam:
                type: host-local
                ranges:
                - - subnet: 10.10.1.0/24
            arguments:
            - fieldPath: master
              valueFrom:
                attributeRef: name
```

### Status

The `status.devices` field is populated by the CNI DRA Driver to reflect the actual state of the network interfaces configured for the given requests in the ResourceClaim.

Since each request corresponds to the configuration of a single network interface, the driver will create a new entry in `status.devices` for each configured interface. The CNI result will be directly reported in the `status.devices.data`. To extract the MAC address, the CNI DRA driver will iterate over the `Interfaces` field in the CNI result in order to find the interface that has an empty `Sandbox` field and that matches the interface name specified in the request.

A condition indicating the network interface has been created successfully will be reported with the `Ready` condition type set to `True`, with the `reason` field set to `NetworkInterfaceReady`, and with the `message` field set with `CNI-DRA-Driver has configured the device.`.

Here is an example below of a full ResourceClaim object with its status:
```yaml
apiVersion: resource.k8s.io/v1beta1
kind: ResourceClaim
metadata:
  name: macvlan-eth0-attachment
...
status:
  allocation:
    devices:
      config:
      - opaque:
          driver: cni.dra.networking.x-k8s.io
          parameters:
            apiVersion: cni.dra.networking.x-k8s.io/v1alpha1
            kind: CNI
            ifName: "net1"
            config:
              cniVersion: 1.0.0
              name: macvlan0
              plugins:
              - type: macvlan
                master: eth0
                mode: bridge
                ipam:
                  type: host-local
                  ranges:
                  - - subnet: 10.10.1.0/24
        requests:
        - macvlan0
        source: FromClaim
      results:
      - device: cni
        driver: cni.dra.networking.x-k8s.io
        pool: kind-worker
        request: macvlan0
    nodeSelector:
      nodeSelectorTerms:
      - matchFields:
        - key: metadata.name
          operator: In
          values:
          - kind-worker
  devices: # filled by the CNI-DRA-Driver. 
  - conditions: 
    - lastHeartbeatTime: "2024-12-16T17:33:45Z"
      lastTransitionTime: "2024-12-14T18:58:57Z"
      message: CNI-DRA-Driver has configured the device.
      reason: NetworkInterfaceReady
      status: "True"
      type: Ready 
    data: # Opaque field containing the CNI result.
    - cniVersion: 1.0.0
      interfaces:
      - mac: b2:af:6a:f9:12:3b
        name: net1
        sandbox: /var/run/netns/cni-d36910c7-c9a4-78f6-abad-26e9a8142a04
      ips:
      - address: 10.10.1.2/24
        gateway: 10.10.1.1
        interface: 0
    device: cni
    driver: cni.dra.networking.x-k8s.io
    networkData:
      ips:
      - 10.10.1.2/24
      hardwareAddress: b2:af:6a:f9:12:3b
      interfaceName: net1
    pool: kind-worker
  reservedFor:
  - name: demo-a
    resource: pods
    uid: 680f0a77-8d0b-4e21-8599-62581e335ed6
```

#### Failures

If the invocation of the CNI `ADD` operation fails, the `DEL` operation will be invoked immediately to clean up the failing interface. An error will also be returned during pod creation to fail the pod creation.

The interface will still be reported in the `status.devices` field of the ResourceClaim but with the `Ready` condition type set to `False`, with the `reason` field set to `NetworkInterfaceNotReady`, and with the `message` field set with the error returned while calling the CNI `ADD` operation.

#### Device/Pool

TBD (Scheduling?)

### Validation

ResourceClaim validation:
* Each request utilizing the device class `cni.networking.x-k8s.io` must have one and only one config associated with it with the driver `cni.dra.networking.x-k8s.io`.
* Each request utilizing the device class `cni.networking.x-k8s.io` must use the allocation mode `ExactCount` with count set to 1.
* A ResourceClaim utilizing the device class `cni.networking.x-k8s.io` must be claimed by one and only one pod.

Opaque Parameter validation:
* All properties in the `CNIConfig` object must be valid (e.g. `IfName`).
* The CNI config must follow correct syntax and semantics.
    * Note: A mechanism is first required from the CNI project to achieve this validation (see [containernetworking/cni#1132](https://github.com/containernetworking/cni/issues/1132)).
* The validation does not check if the CNI plugin exists (This responsibility is on the scheduler)

## Related Resources

* [cni.dev](https://www.cni.dev/) 
* [k8snetworkplumbingwg/multus-cni](https://github.com/k8snetworkplumbingwg/multus-cni)
* [KEP-4817 - Resource Claim Status with possible standardized network interface data](https://github.com/kubernetes/enhancements/issues/4817)
* [containernetworking/cni#1132 - VALIDATE Operation](https://github.com/containernetworking/cni/issues/1132)

## Alternatives

### CNI configuration via a separate resource

Allowing users to configure CNI poses significant security risks, including privilege escalation, network isolation breaches, denial-of-service attacks, data exfiltration, and execution of arbitrary code, especially since CNIs often run with elevated privileges and control critical network settings. Misconfigurations or use of untrusted plugins can lead to IP spoofing, unauthorized network access, or malware introduction. To mitigate these risks, CNI configuration should be tightly controlled, limited to trusted users, validated through policies, and restricted to approved plugins to maintain secure and reliable container networking.

Instead of embedding CNI configuration directly within a ResourceClaim, which introduces security and control concerns, we can define a separate, dedicated API (e.g., CNIConfig) to manage and validate CNI configurations centrally. The ResourceClaim can then reference the desired CNI setup by name through a parameter (e.g., configName), allowing users to select from pre-approved configurations without having direct control over the CNI spec. This approach enhances security, simplifies validation, and enables administrators to maintain tighter control over available networking options.

For example:

```yaml
kind: CNIConfig
metadata:
  name: macvlan-host-local
spec:
  apiVersion: cni.networking.x-k8s.io/v1alpha1
  kind: CNI
  ifName: "net1"
  cniVersion: 1.0.0
  plugins:
  - type: macvlan
    config:
      mode: bridge
      ipam:
        type: host-local
        ranges:
        - - subnet: 10.10.1.0/24
    arguments:
    - fieldPath: master
      valueFrom:
      attributeRef: name
```

```yaml
apiVersion: resource.k8s.io/v1beta1
kind: ResourceClaim
metadata:
  name: one-macvlan-attachment
spec:
  devices:
    requests:
    - name: macvlan0
      deviceClassName: cni.networking.x-k8s.io
      allocationMode: ExactCount
      count: 1
    config:
    - requests:
      - macvlan0
      opaque:
        driver: cni.dra.networking.x-k8s.io
        parameters:
          configName: macvlan-host-local
```
