/*
 * The MIT License (MIT)
 * Copyright (c) 2016 Michael Franz Aigner <maigner@updox.com>
 * Copyright (c) 2016 Maksym Borodin <borodin.maksym@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated
 * documentation files (the "Software"), to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software,
 * and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial portions
 * of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO
 * THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF
 * CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 */
package xenserver

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	xenapi "github.com/terra-farm/go-xen-api-client"
)

const (
	vmSchemaNameLabel                 = "name_label"
	vmSchemaBaseTemplateName          = "base_template_name"
	vmSchemaStaticMemoryMin           = "static_mem_min"
	vmSchemaStaticMemoryMax           = "static_mem_max"
	vmSchemaDynamicMemoryMin          = "dynamic_mem_min"
	vmSchemaDynamicMemoryMax          = "dynamic_mem_max"
	vmSchemaBootOrder                 = "boot_order"
	vmSchemaNetworkInterfaces         = "network_interface"
	vmSchemaHardDrive                 = "hard_drive"
	vmSchemaCdRom                     = "cdrom"
	vmSchemaBootParameters            = "boot_parameters"
	vmSchemaInstallationMediaType     = "installation_media_type"
	vmSchemaInstallationMediaLocation = "installation_media_location"
	vmSchemaVcpus                     = "vcpus"
	vmSchemaCoresPerSocket            = "cores_per_socket"
	vmSchemaXenstoreData              = "xenstore_data"
	vmSchemaOtherConfig               = "other_config"
)

func resourceVM() *schema.Resource {
	return &schema.Resource{
		Create: resourceVMCreate,
		Read:   resourceVMRead,
		Update: resourceVMUpdate,
		Delete: resourceVMDelete,
		Exists: resourceVMExists,

		Schema: map[string]*schema.Schema{
			vmSchemaNameLabel: &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			vmSchemaBaseTemplateName: &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			vmSchemaXenstoreData: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Default:  nil,
				Computed: true,
			},

			vmSchemaStaticMemoryMin: &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			vmSchemaStaticMemoryMax: &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			vmSchemaDynamicMemoryMin: &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			vmSchemaDynamicMemoryMax: &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			vmSchemaBootOrder: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "dc",
			},

			vmSchemaNetworkInterfaces: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     resourceVIF(),
				Set:      vifHash,
			},

			vmSchemaHardDrive: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     resourceVBD(),
				Set:      vbdHash,
			},

			vmSchemaCdRom: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     resourceVBD(),
				Set:      vbdHash,
			},

			vmSchemaBootParameters: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},

			vmSchemaInstallationMediaType: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},

			vmSchemaInstallationMediaLocation: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},

			vmSchemaVcpus: &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			vmSchemaCoresPerSocket: &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},

			vmSchemaOtherConfig: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
			},
		},
	}
}

func filterVMTemplates(c *Connection, vms []xenapi.VMRef) ([]xenapi.VMRef, error) {
	var templates []xenapi.VMRef
	for _, vm := range vms {
		isATemplate, err := c.client.VM.GetIsATemplate(c.session, vm)
		if err != nil {
			return templates, err
		}
		if isATemplate {
			templates = append(templates, vm)
		}
	}
	return templates, nil
}

func resourceVMCreate(d *schema.ResourceData, m interface{}) error {
	c := m.(*Connection)
	d.Partial(true)

	dBaseTemplateName := d.Get(vmSchemaBaseTemplateName).(string)

	xenBaseTemplates, err := c.client.VM.GetByNameLabel(c.session, dBaseTemplateName)
	if err != nil {
		return err
	}

	xenBaseTemplates, err = filterVMTemplates(c, xenBaseTemplates)
	if err != nil {
		return err
	}

	if len(xenBaseTemplates) == 0 {
		return fmt.Errorf("no VM template with label %q has been found", dBaseTemplateName)
	}

	if len(xenBaseTemplates) > 1 {
		return fmt.Errorf("more than one VM template with label %q has been found", dBaseTemplateName)
	}

	xenBaseTemplate := xenBaseTemplates[0]

	dNameLabel := d.Get(vmSchemaNameLabel).(string)

	xenVM, err := c.client.VM.Clone(c.session, xenBaseTemplate, dNameLabel)
	if err != nil {
		return err
	}

	vm := &VMDescriptor{
		VMRef: xenVM,
	}

	if err = vm.Query(c); err != nil {
		return err
	}

	d.SetPartial(vmSchemaNameLabel)
	d.SetId(vm.UUID)

	otherConfig := vm.OtherConfig
	for k, v := range d.Get(vmSchemaOtherConfig).(map[string]interface{}) {
		otherConfig[k] = v.(string)
	}

	// Reset base template name
	otherConfig["base_template_name"] = dBaseTemplateName

	if err = c.client.VM.SetOtherConfig(c.session, vm.VMRef, otherConfig); err != nil {
		return err
	}

	// Memory configuration
	updatedFields := make([]string, 0, 5)
	mem, ok := d.GetOk(vmSchemaStaticMemoryMin)
	if ok {
		vm.StaticMemory.Min = mem.(int)
		updatedFields = append(updatedFields, vmSchemaStaticMemoryMin)
	}

	mem, ok = d.GetOk(vmSchemaStaticMemoryMax)
	if ok {
		vm.StaticMemory.Max = mem.(int)
		updatedFields = append(updatedFields, vmSchemaStaticMemoryMax)
	}

	mem, ok = d.GetOk(vmSchemaDynamicMemoryMin)
	if ok {
		vm.DynamicMemory.Min = mem.(int)
		updatedFields = append(updatedFields, vmSchemaDynamicMemoryMin)
	}

	mem, ok = d.GetOk(vmSchemaDynamicMemoryMax)
	if ok {
		vm.DynamicMemory.Max = mem.(int)
		updatedFields = append(updatedFields, vmSchemaDynamicMemoryMax)
	}

	if len(updatedFields) > 0 {
		if err = vm.UpdateMemory(c); err != nil {
			return err
		}
		for _, f := range updatedFields {
			d.SetPartial(f)
		}
		updatedFields = make([]string, 0, 5)
	}

	// Set VCPUs number
	vm.VCPUCount = d.Get(vmSchemaVcpus).(int)
	if err = vm.UpdateVCPUs(c); err != nil {
		return err
	} else {
		d.SetPartial(vmSchemaVcpus)
	}

	dXenstoreDataRaw, ok := d.GetOk(vmSchemaXenstoreData)
	if ok && dXenstoreDataRaw != nil {
		vm.XenstoreData = make(map[string]string)
		for key, value := range dXenstoreDataRaw.(map[string]interface{}) {
			vm.XenstoreData[key] = value.(string)
		}

		err = c.client.VM.SetXenstoreData(c.session, vm.VMRef, vm.XenstoreData)
		if err != nil {
			return err
		} else {
			d.SetPartial(vmSchemaXenstoreData)
		}
	}

	if vm.XenstoreData, err = c.client.VM.GetXenstoreData(c.session, vm.VMRef); err != nil {
		return err
	}
	err = d.Set(vmSchemaXenstoreData, vm.XenstoreData)
	if err != nil {
		return err
	}

	log.Println("[DEBUG] VM Power State: ", vm.PowerState)

	var vifs []*VIFDescriptor

	if vifs, err = readVIFsFromSchema(c, d.Get(vmSchemaNetworkInterfaces).(*schema.Set).List()); err != nil {
		return err
	}

	for _, vif := range vifs {
		vif.VM = vm
		if vif, err = createVIF(c, vif); err != nil {
			log.Println("[ERROR] ", err)
			return err
		}
	}
	d.SetPartial(vmSchemaNetworkInterfaces)

	log.Println("[DEBUG] Creating CDs")
	if err = createVBDs(c, d.Get(vmSchemaCdRom).(*schema.Set).List(), xenapi.VbdTypeCD, vm); err != nil {
		log.Println("[ERROR] ", err)
		return err
	} else {
		updatedFields = append(updatedFields, vmSchemaCdRom)
	}

	log.Println("[DEBUG] Creating HDDs")
	if err = createVBDs(c, d.Get(vmSchemaHardDrive).(*schema.Set).List(), xenapi.VbdTypeDisk, vm); err != nil {
		log.Println("[ERROR] ", err)
		return err
	} else {
		updatedFields = append(updatedFields, vmSchemaHardDrive)
	}

	if setSchemaVBDs(c, vm, d) != nil {
		log.Println("[ERROR] ", err)
		return err
	} else {
		for _, f := range updatedFields {
			d.SetPartial(f)
		}
		updatedFields = make([]string, 0, 5)
	}

	if _order, ok := d.GetOk(vmSchemaBootOrder); ok {
		order := _order.(string)
		vm.HVMBootParameters["order"] = order
	}

	if err = c.client.VM.SetHVMBootParams(c.session, vm.VMRef, vm.HVMBootParameters); err != nil {
		return err
	} else {
		d.SetPartial(vmSchemaBootOrder)
	}

	if _coresPerSocket, ok := d.GetOk(vmSchemaCoresPerSocket); ok {
		coresPerSocket := _coresPerSocket.(int)

		if vm.VCPUCount%coresPerSocket != 0 {
			return fmt.Errorf("%d cores could not fit to %d cores-per-socket topology", vm.VCPUCount, coresPerSocket)
		}

		vm.Platform["cores-per-socket"] = strconv.Itoa(coresPerSocket)
	} else {
		_coresPerSocket = vm.Platform["cores-per-socket"]
		// If empty - set one core per socket
		if _coresPerSocket == "" {
			_coresPerSocket = "1"
		}

		var coresPerSocket int
		if coresPerSocket, err = strconv.Atoi(_coresPerSocket.(string)); err != nil {
			if err = d.Set(vmSchemaCoresPerSocket, coresPerSocket); err != nil {
				return err
			}
		}
	}

	if err = c.client.VM.SetPlatform(c.session, vm.VMRef, vm.Platform); err != nil {
		return err
	} else {
		d.SetPartial(vmSchemaCoresPerSocket)
	}

	log.Println("[DEBUG] Provisioning VM")
	err = c.client.VM.Provision(c.session, xenVM)
	if err != nil {
		return err
	}

	// reset template flag
	if vm.IsATemplate {
		if err = c.client.VM.SetIsATemplate(c.session, vm.VMRef, false); err != nil {
			return err
		}
	}

	d.Partial(false)

	// TODO: Seems like this is more about the state of the resource than the creation of the resource?
	log.Println("[DEBUG] Starting VM")
	err = c.client.VM.Start(c.session, xenVM, false, false)
	if err != nil {
		return err
	}
	log.Println("[DEBUG] Done")

	return nil
}

func resourceVMRead(d *schema.ResourceData, m interface{}) error {
	c := m.(*Connection)

	vm := &VMDescriptor{
		UUID: d.Id(),
	}
	if err := vm.Load(c); err != nil {
		if xenErr, ok := err.(*xenapi.Error); ok {
			if xenErr.Code() == xenapi.ERR_UUID_INVALID {
				d.SetId("")
				return nil
			}
		}

		return err
	}

	err := d.Set(vmSchemaNameLabel, vm.Name)
	if err != nil {
		return err
	}

	vmBaseTemplateName, ok := vm.OtherConfig["base_template_name"]
	if ok {
		err = d.Set(vmSchemaBaseTemplateName, vmBaseTemplateName)
		if err != nil {
			return err
		}
	}

	err = d.Set(vmSchemaXenstoreData, vm.XenstoreData)
	if err != nil {
		return err
	}

	err = d.Set(vmSchemaVcpus, vm.VCPUCount)
	if err != nil {
		return err
	}

	err = d.Set(vmSchemaStaticMemoryMax, vm.StaticMemory.Max)
	if err != nil {
		return err
	}

	err = d.Set(vmSchemaStaticMemoryMin, vm.StaticMemory.Min)
	if err != nil {
		return err
	}

	err = d.Set(vmSchemaDynamicMemoryMax, vm.DynamicMemory.Max)
	if err != nil {
		return err
	}

	err = d.Set(vmSchemaDynamicMemoryMin, vm.DynamicMemory.Min)
	if err != nil {
		return err
	}

	vmVifs, err := c.client.VM.GetVIFs(c.session, vm.VMRef)
	if err != nil {
		return err
	}

	vifs := make([]map[string]interface{}, 0, len(vmVifs))
	log.Println(fmt.Sprintf("[DEBUG] Got %d VIFs", len(vmVifs)))

	for _, _vif := range vmVifs {
		vif := VIFDescriptor{
			VIFRef: _vif,
		}

		if err := vif.Query(c); err != nil {
			return err
		}

		log.Println("[DEBUG] Found VIF", vif.UUID)
		vifData := fillVIFSchema(vif)
		log.Println("[DEBUG] VIF: ", vifData)

		vifs = append(vifs, vifData)
	}
	err = d.Set(vmSchemaNetworkInterfaces, vifs)
	if err != nil {
		log.Println("[ERROR] ", err)
		return err
	}

	if setSchemaVBDs(c, vm, d) != nil {
		log.Println("[ERROR] ", err)
		return err
	}

	log.Println("[DEBUG] Query boot order")
	if order, ok := vm.HVMBootParameters["order"]; ok {
		if err := d.Set(vmSchemaBootOrder, order); err != nil {
			return err
		}
	}

	if cps, ok := vm.Platform["cores-per-socket"]; ok {
		coresPerSocket, _ := strconv.Atoi(cps)
		if err := d.Set(vmSchemaCoresPerSocket, coresPerSocket); err != nil {
			return err
		}
	}

	return nil
}

func resourceVMUpdate(d *schema.ResourceData, m interface{}) error {
	c := m.(*Connection)

	vm := &VMDescriptor{
		UUID: d.Id(),
	}
	if err := vm.Load(c); err != nil {
		if xenErr, ok := err.(*xenapi.Error); ok {
			if xenErr.Code() == xenapi.ERR_UUID_INVALID {
				d.SetId("")
				return nil
			}
		}

		return err
	}

	d.Partial(true)

	if d.HasChange(vmSchemaNameLabel) {
		_, _dNameLabel := d.GetChange(vmSchemaNameLabel)
		dNameLabel := _dNameLabel.(string)
		if err := c.client.VM.SetNameLabel(c.session, vm.VMRef, dNameLabel); err != nil {
			return err
		}

		d.SetPartial(vmSchemaNameLabel)
	}

	updatedFields := make([]string, 0, 5)
	updateMemory := false

	if d.HasChange(vmSchemaStaticMemoryMax) {
		mem := d.Get(vmSchemaStaticMemoryMax).(int)
		vm.StaticMemory.Max = mem
		updateMemory = true
		updatedFields = append(updatedFields, vmSchemaStaticMemoryMax)
	}

	if d.HasChange(vmSchemaStaticMemoryMin) {
		mem := d.Get(vmSchemaStaticMemoryMin).(int)
		vm.StaticMemory.Min = mem
		updateMemory = true
		updatedFields = append(updatedFields, vmSchemaStaticMemoryMin)
	}

	if d.HasChange(vmSchemaDynamicMemoryMax) {
		mem := d.Get(vmSchemaDynamicMemoryMax).(int)
		vm.DynamicMemory.Max = mem
		updateMemory = true
		updatedFields = append(updatedFields, vmSchemaDynamicMemoryMax)
	}

	if d.HasChange(vmSchemaDynamicMemoryMin) {
		mem := d.Get(vmSchemaDynamicMemoryMin).(int)
		vm.DynamicMemory.Min = mem
		updateMemory = true
		updatedFields = append(updatedFields, vmSchemaDynamicMemoryMin)
	}

	if updateMemory {
		if err := vm.UpdateMemory(c); err != nil {
			return err
		}

		for _, f := range updatedFields {
			d.SetPartial(f)
		}
	}

	if d.HasChange(vmSchemaVcpus) {
		_, vcpus := d.GetChange(vmSchemaVcpus)
		vm.VCPUCount = vcpus.(int)
		if err := vm.UpdateVCPUs(c); err != nil {
			return err
		}
		d.SetPartial(vmSchemaVcpus)
	}

	if d.HasChange(vmSchemaNetworkInterfaces) {
		o, n := d.GetChange(vmSchemaNetworkInterfaces)

		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		var err error
		var remove []*VIFDescriptor
		if remove, err = readVIFsFromSchema(c, os.Difference(ns).List()); err != nil {
			return err
		}

		if len(remove) > 0 {

			log.Println(fmt.Sprintf("[DEBUG] Got %d VIFs to remove", len(remove)))

			var vmVifs []*VIFDescriptor
			if _vmVifs, err := c.client.VM.GetVIFs(c.session, vm.VMRef); err == nil {
				for _, _vif := range _vmVifs {
					vif := &VIFDescriptor{
						VIFRef: _vif,
					}

					if err := vif.Query(c); err != nil {
						return err
					}
					vmVifs = append(vmVifs, vif)
				}
			} else {
				return err
			}

			for _, vif := range remove {
				var vifToRemove *VIFDescriptor
				for _, candidate := range vmVifs {
					if candidate.Network.UUID == vif.Network.UUID && candidate.DeviceOrder == vif.DeviceOrder {
						vifToRemove = candidate
						break
					}
				}
				if vifToRemove != nil {
					log.Println(fmt.Sprintf("[DEBUG] Removing VIF %q", vif.UUID))
					if err := c.client.VIF.Destroy(c.session, vifToRemove.VIFRef); err != nil {
						return err
					}

					// Decrement VIFCount after removal success
					vm.VIFCount--
				}
			}
		}

		var create []*VIFDescriptor
		if create, err = readVIFsFromSchema(c, ns.Difference(os).List()); err != nil {
			return err
		}

		if len(create) > 0 {
			log.Println(fmt.Sprintf("[DEBUG] Will create %d VIFs", len(create)))
			for _, vif := range create {
				vif.VM = vm
				if _, err := createVIF(c, vif); err != nil {
					return nil
				}
			}
		}
		d.SetPartial(vmSchemaNetworkInterfaces)
	}

	if d.HasChange(vmSchemaCdRom) {
		o, n := d.GetChange(vmSchemaCdRom)

		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		var err error
		var remove []*VBDDescriptor
		if remove, err = readVBDsFromSchema(c, os.Difference(ns).List()); err == nil {
			return err
		}

		if len(remove) > 0 {

			log.Println(fmt.Sprintf("[DEBUG] Got %d cdroms to remove", len(remove)))

			var vmVBDs []*VBDDescriptor
			if _vmVBDs, err := c.client.VM.GetVBDs(c.session, vm.VMRef); err == nil {
				for _, _vbd := range _vmVBDs {
					vbd := &VBDDescriptor{
						VBDRef: _vbd,
					}

					if err := vbd.Query(c); err != nil {
						return err
					}
					vmVBDs = append(vmVBDs, vbd)
				}
			} else {
				return err
			}

			for _, vbd := range remove {
				var vbdToRemove *VBDDescriptor
				for _, candidate := range vmVBDs {
					if candidate.UserDevice == vbd.UserDevice {
						vbdToRemove = candidate
						break
					}
				}
				if vbdToRemove != nil {
					log.Println(fmt.Sprintf("[DEBUG] Removing cdrom %q", vbd.UUID))
					if err := c.client.VBD.Destroy(c.session, vbdToRemove.VBDRef); err != nil {
						return err
					}
				}
			}
		}

		var create []*VBDDescriptor
		if create, err = readVBDsFromSchema(c, ns.Difference(os).List()); err == nil {
			return err
		}

		if len(create) > 0 {
			log.Println(fmt.Sprintf("[DEBUG] Will create %d cdroms", len(create)))
			for _, cdrom := range create {
				cdrom.VM = vm
				if _, err := createVBD(c, cdrom); err != nil {
					return err
				}
			}
		}
		d.SetPartial(vmSchemaCdRom)
	}

	if d.HasChange(vmSchemaHardDrive) {
		o, n := d.GetChange(vmSchemaHardDrive)

		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		var err error
		var remove []*VBDDescriptor
		if remove, err = readVBDsFromSchema(c, os.Difference(ns).List()); err == nil {
			return err
		}

		if len(remove) > 0 {

			log.Println(fmt.Sprintf("[DEBUG] Got %d HDDs to remove", len(remove)))

			var vmVBDs []*VBDDescriptor
			if _vmVBDs, err := c.client.VM.GetVBDs(c.session, vm.VMRef); err == nil {
				for _, _vbd := range _vmVBDs {
					vbd := &VBDDescriptor{
						VBDRef: _vbd,
					}

					if err := vbd.Query(c); err != nil {
						return err
					}
					vmVBDs = append(vmVBDs, vbd)
				}
			} else {
				return err
			}

			for _, vbd := range remove {
				var vbdToRemove *VBDDescriptor
				for _, candidate := range vmVBDs {
					if candidate.UserDevice == vbd.UserDevice {
						vbdToRemove = candidate
						break
					}
				}
				if vbdToRemove != nil {
					log.Println(fmt.Sprintf("[DEBUG] Removing HDD %q", vbd.UUID))
					if err := c.client.VBD.Destroy(c.session, vbdToRemove.VBDRef); err != nil {
						return err
					}
				}
			}
		}

		var create []*VBDDescriptor
		if create, err = readVBDsFromSchema(c, ns.Difference(os).List()); err == nil {
			return err
		}

		if len(create) > 0 {
			log.Println(fmt.Sprintf("[DEBUG] Will create %d HDDs", len(create)))
			for _, hdd := range create {
				hdd.VM = vm
				if _, err := createVBD(c, hdd); err != nil {
					return err
				}
			}
		}
	}

	dXenstoreDataRaw, ok := d.GetOk(vmSchemaXenstoreData)
	if ok {
		dXenstoreData := make(map[string]string)
		for key, value := range dXenstoreDataRaw.(map[string]interface{}) {
			dXenstoreData[key] = value.(string)
		}

		if err := c.client.VM.SetXenstoreData(c.session, vm.VMRef, dXenstoreData); err != nil {
			return err
		}

		d.SetPartial(vmSchemaXenstoreData)
	}

	if d.HasChange(vmSchemaBootOrder) {
		_, n := d.GetChange(vmSchemaBootOrder)
		order := n.(string)
		vm.HVMBootParameters["order"] = order

		if err := c.client.VM.SetHVMBootParams(c.session, vm.VMRef, vm.HVMBootParameters); err != nil {
			return err
		}

		d.SetPartial(vmSchemaBootOrder)
	}

	if d.HasChange(vmSchemaCoresPerSocket) {
		_, n := d.GetChange(vmSchemaCoresPerSocket)
		coresPerSocket := n.(int)

		if vm.VCPUCount%coresPerSocket != 0 {
			return fmt.Errorf("%d cores could not fit to %d cores-per-socket topology", vm.VCPUCount, coresPerSocket)
		}

		vm.Platform["cores-per-socket"] = strconv.Itoa(coresPerSocket)

		if err := c.client.VM.SetPlatform(c.session, vm.VMRef, vm.Platform); err != nil {
			return err
		}

		d.SetPartial(vmSchemaCoresPerSocket)
	}

	d.Partial(false)

	return resourceVMRead(d, m)
}

func resourceVMDelete(d *schema.ResourceData, m interface{}) error {
	c := m.(*Connection)

	vm := VMDescriptor{
		UUID: d.Id(),
	}
	if err := vm.Load(c); err != nil {
		if xenErr, ok := err.(*xenapi.Error); ok {
			if xenErr.Code() == xenapi.ERR_UUID_INVALID {
				d.SetId("")
				return nil
			}
		}

		return err
	}

	if vm.PowerState == xenapi.VMPowerStateRunning {
		if err := c.client.VM.HardShutdown(c.session, vm.VMRef); err != nil {
			return err
		}
	}

	vifs, err := c.client.VM.GetVIFs(c.session, vm.VMRef)
	if err != nil {
		return err
	}

	for _, vif := range vifs {
		if err := c.client.VIF.Destroy(c.session, vif); err != nil {
			return err
		}
	}

	var vbds []*VBDDescriptor
	if vbds, err = queryTemplateVBDs(c, &vm); err != nil {
		return err
	}
	log.Printf("[DEBUG] Found %d template vbds", len(vbds))

	if err := c.client.VM.Destroy(c.session, vm.VMRef); err != nil {
		return err
	}

	if err = destroyTemplateVDIs(c, vbds); err != nil {
		return err
	}

	d.SetId("")
	return nil
}

func resourceVMExists(d *schema.ResourceData, m interface{}) (bool, error) {
	c := m.(*Connection)

	_, err := c.client.VM.GetByUUID(c.session, d.Id())
	if err != nil {
		if xenErr, ok := err.(*xenapi.Error); ok {
			if xenErr.Code() == xenapi.ERR_UUID_INVALID {
				return false, nil
			}
		}

		return false, err
	}

	return true, nil
}
