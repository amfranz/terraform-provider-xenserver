---
title: "xenserver_network"
---

Provides information about the Networks of a XenServer host.

## Example Usage

```hcl
data "xenserver_network" "vlan-100" {
  name_label = "VLAN_100"
}

resource "xenserver_vm" "web" {
    name_label = "web"
    base_template_name = "<desired template>"
    static_mem_min = 2147483648 # 2GB
    static_mem_max = 2147483648
    dynamic_mem_min = 2147483648
    dynamic_mem_max = 2147483648
    boot_order = "cdn"
    network_interface {
        network_uuid = "<uuid>"
        mac = "<desired-mac>"
        mtu = 1500
        device = 0
    }
    network_interface {
        network_uuid = "${data.xenserver_network.vlan-100.id}"
        mtu = 1500
        device = 1
    }
    vcpus = 2
    cdrom {
      vdi_uuid = "<iso uuid>"
    }
    hard_drive {
      vdi_uuid = "<desired vdi>"
    }
}
```
