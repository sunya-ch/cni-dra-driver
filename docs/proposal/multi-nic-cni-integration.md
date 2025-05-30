# Multi-NIC CNI Integration

The key feature of multi-nic-cni is to handle multiple network configurations at the same time in a harmony manner. 

## Capability of multi-nic-cni

### Interface selection
- Discover and select with multi-nic daemon.

#### Potentials with DRA
- use DRA with CEL, prioritized alternatives ,and consumable capcity.

### Dynamic configuration
- configure IP address of Pod network corresponding on selected network interface.
  currently, it call multi-nic-cni-ipam with a set of configurations instead of single configuration and replace IPs with static IPAM for each configuration.

##### Potentials with DRA
- interfaces are selected on the fly.
- CNI DRA driver may include the following parameters in the resource claim template and apply the template values with the scheduled node and allocated interfaces.

```yaml
template: |
    {
        "cniVersion": "0.4.0",
        "name": "macvlan-network",
        "type": "macvlan",
        "master": "{{ .interfaceName }}",
        "mode": "bridge",
        "ipam": {
            "type": "host-local",
            "subnet": "{{ .host.interface.subnet }}",
            "rangeStart": "{{ .host.interface.rangeStart }}",
            "rangeEnd": "{{ .host.interface.rangeEnd }}",
            "routes": [
            { "dst": "0.0.0.0/0" }
            ],
            "gateway": "{{ .host.interface.gateway }}"
        }
    }
hostConfigs:
  default:
  - subnet: "192.168.100.0/24"
  - gateway: "192.168.100.1"
  hosts:
  - name: worker1
    interfaces:
      eth1:
        subnet: "192.168.100.0/24"
        rangeStart: "192.168.100.100"
        rangeEnd: "192.168.100.200"
      eth2:
        subnet: "192.168.200.0/24"
        rangeStart: "192.168.200.100"
        rangeEnd: "192.168.200.200"
        gateway: "192.168.200.1"
  - name: worker2
    default:
        subnet: "192.168.100.0/24"
        rangeStart: "192.168.100.201"
        rangeEnd: "192.168.100.240"
  - name: worker3
    default:
        subnet: "192.168.200.0/24"
        rangeStart: "192.168.200.201"
        rangeEnd: "192.168.200.240"
        gateway: "192.168.200.1"
```

The missing piece to fill the gap is the missing CNIs:
- host-local-ipam
    - compute and manage CIDR to dynamically define corresponding host-inteface-local config
- multi-gateway route in route config
- aws-ipvlan-ipam