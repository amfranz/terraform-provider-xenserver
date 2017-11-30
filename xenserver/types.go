/*
 * The MIT License (MIT)
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

	"github.com/ringods/go-xen-api-client"
)

type Range struct {
	Min int
	Max int
}

type NetworkDescriptor struct {
	UUID        string
	Name        string
	Description string
	Bridge      string
	MTU         int

	NetworkRef xenAPI.NetworkRef
}

type VMDescriptor struct {
	UUID              string
	Name              string
	Description       string
	PowerState        xenAPI.VMPowerState
	IsPV              bool
	StaticMemory      Range
	DynamicMemory     Range
	VCPUCount         int
	VIFCount          int
	VBDCount          int
	PCICount          int
	OtherConfig       map[string]string
	XenstoreData      map[string]string
	HVMBootParameters map[string]string
	Platform          map[string]string
	IsATemplate       bool

	VMRef xenAPI.VMRef
}

type VIFDescriptor struct {
	Network            *NetworkDescriptor
	VM                 *VMDescriptor
	UUID               string
	MTU                int
	MAC                string
	IsAutogeneratedMAC bool
	DeviceOrder        int
	OtherConfig        map[string]string

	VIFRef xenAPI.VIFRef
}

type SRDescriptor struct {
	Name        string
	UUID        string
	Description string
	Host        string
	Type        string
	ContentType string
	Shared      bool

	SRRef xenAPI.SRRef
}

type VDIDescriptor struct {
	Name       string
	UUID       string
	SR         *SRDescriptor
	IsShared   bool
	IsReadOnly bool
	Size       int

	VDIRef xenAPI.VDIRef
}

type VBDDescriptor struct {
	UUID             string
	VM               *VMDescriptor
	VDI              *VDIDescriptor
	Device           string
	UserDevice       string
	Mode             xenAPI.VbdMode
	Type             xenAPI.VbdType
	Bootable         bool
	OtherConfig      map[string]string
	IsTemplateDevice bool

	VBDRef xenAPI.VBDRef
}

type PIFDescriptor struct {
	UUID string

	PIFRef xenAPI.PIFRef
}

type VLANDescriptor struct {
	UUID        string
	Tag         int
	TaggedPIF   PIFDescriptor
	UntaggedPIF PIFDescriptor
	OtherConfig map[string]string

	VLANRef xenAPI.VLANRef
}

func (this *NetworkDescriptor) Load(c *Connection) error {
	var network xenAPI.NetworkRef

	hasNetName := false
	hasNetUUID := false

	if this.Name != "" {
		networks, err := c.client.Network.GetByNameLabel(c.session, this.Name)
		if err != nil {
			return err
		}

		if len(networks) == 0 {
			return fmt.Errorf("Network %q not found!", this.Name)
		}

		hasNetName = true
		network = networks[0]
	}

	if !hasNetName {
		if this.UUID != "" {
			_network, err := c.client.Network.GetByUUID(c.session, this.UUID)
			if err != nil {
				return err
			}
			hasNetUUID = true
			network = _network
		}
	}

	if !hasNetName && !hasNetUUID {
		return fmt.Errorf("%q should be specified!", vifSchemaNetworkUUID)
	}

	this.NetworkRef = network

	return this.Query(c)
}

func (this *NetworkDescriptor) Query(c *Connection) error {
	network, err := c.client.Network.GetRecord(c.session, this.NetworkRef)
	if err != nil {
		return err
	}

	this.UUID = network.UUID
	this.Name = network.NameLabel
	this.Description = network.NameDescription
	this.MTU = network.MTU
	this.Bridge = network.Bridge

	return nil
}

func (this *VMDescriptor) Load(c *Connection) error {
	var vm xenAPI.VMRef

	hasVMName := false
	hasVMUUID := false

	if this.Name != "" {
		vms, err := c.client.VM.GetByNameLabel(c.session, this.Name)
		if err != nil {
			return err
		}

		if len(vms) == 0 {
			return fmt.Errorf("VM %q not found!", this.Name)
		}

		hasVMName = true
		vm = vms[0]
	}

	if !hasVMName {
		if this.UUID != "" {
			_vm, err := c.client.VM.GetByUUID(c.session, this.UUID)
			if err != nil {
				return err
			}
			hasVMUUID = true
			vm = _vm
		}
	}

	if !hasVMName && !hasVMUUID {
		return fmt.Errorf("Either name or UUID should be specified!")
	}

	this.VMRef = vm

	return this.Query(c)
}

func (this *VMDescriptor) Query(c *Connection) error {
	vm, err := c.client.VM.GetRecord(c.session, this.VMRef)
	if err != nil {
		return err
	}

	this.UUID = vm.UUID
	this.Name = vm.NameLabel
	this.Description = vm.NameDescription
	this.PowerState = vm.PowerState
	this.IsPV = vm.PVBootloader != ""
	this.VCPUCount = vm.VCPUsMax
	this.StaticMemory = Range{
		Min: vm.MemoryStaticMin,
		Max: vm.MemoryStaticMax,
	}
	this.DynamicMemory = Range{
		Min: vm.MemoryDynamicMin,
		Max: vm.MemoryDynamicMax,
	}
	this.VIFCount = len(vm.VIFs)
	this.VBDCount = len(vm.VBDs)
	this.PCICount = len(vm.AttachedPCIs)
	this.OtherConfig = vm.OtherConfig
	this.XenstoreData = vm.XenstoreData
	this.HVMBootParameters = vm.HVMBootParams
	this.IsATemplate = vm.IsATemplate

	if this.Platform, err = c.client.VM.GetPlatform(c.session, this.VMRef); err != nil {
		return err
	}

	return nil
}

func (this *VMDescriptor) UpdateMemory(c *Connection) error {
	return c.client.VM.SetMemoryLimits(c.session,
		this.VMRef,
		this.StaticMemory.Min,
		this.StaticMemory.Max,
		this.DynamicMemory.Min,
		this.DynamicMemory.Max)
}

func (this *VMDescriptor) UpdateVCPUs(c *Connection) error {
	if err := c.client.VM.SetVCPUsMax(c.session, this.VMRef, this.VCPUCount); err != nil {
		return err
	}
	if err := c.client.VM.SetVCPUsAtStartup(c.session, this.VMRef, this.VCPUCount); err != nil {
		return err
	}

	return nil
}

func (this *VIFDescriptor) Load(c *Connection) error {
	var VIFRef xenAPI.VIFRef
	var err error
	if VIFRef, err = c.client.VIF.GetByUUID(c.session, this.UUID); err != nil {
		return err
	}
	this.VIFRef = VIFRef

	return this.Query(c)
}

func (this *VIFDescriptor) Query(c *Connection) error {
	var vif xenAPI.VIFRecord
	var err error
	if vif, err = c.client.VIF.GetRecord(c.session, this.VIFRef); err != nil {
		return err
	}

	this.UUID = vif.UUID
	this.MTU = vif.MTU
	this.DeviceOrder, err = strconv.Atoi(vif.Device) // Error ignored, should not occur
	this.IsAutogeneratedMAC = vif.MACAutogenerated
	this.MAC = vif.MAC
	this.OtherConfig = vif.OtherConfig

	if this.Network == nil {
		this.Network = &NetworkDescriptor{
			NetworkRef: vif.Network,
		}
		if err := this.Network.Query(c); err != nil {
			return err
		}
	}

	if this.VM == nil {
		this.VM = &VMDescriptor{
			VMRef: vif.VM,
		}
		if err := this.VM.Query(c); err != nil {
			return err
		}
	}

	return nil
}

func (this *SRDescriptor) Load(c *Connection) error {
	var sr xenAPI.SRRef

	hasSRName := false
	hasSRUUID := false

	if this.Name != "" {
		srs, err := c.client.SR.GetByNameLabel(c.session, this.Name)
		if err != nil {
			return err
		}

		if len(srs) == 0 {
			return fmt.Errorf("Storage repository %q not found!", this.Name)
		}

		hasSRName = true
		sr = srs[0]
	}

	if !hasSRName {
		if this.UUID != "" {
			_sr, err := c.client.SR.GetByUUID(c.session, this.UUID)
			if err != nil {
				return err
			}
			hasSRUUID = true
			sr = _sr
		}
	}

	if !hasSRName && !hasSRUUID {
		return fmt.Errorf("Either %q or %q should be specified!", srSchemaName, srSchemaUUID)
	}

	this.SRRef = sr

	return this.Query(c)
}

func (this *SRDescriptor) Query(c *Connection) error {
	sr, err := c.client.SR.GetRecord(c.session, this.SRRef)
	if err != nil {
		return err
	}

	this.UUID = sr.UUID
	this.Name = sr.NameLabel
	this.Description = sr.NameDescription
	this.Shared = sr.Shared
	this.Type = sr.Type
	this.ContentType = sr.ContentType
	log.Println("[DEBUG] ", sr.SmConfig)

	return nil
}

func (this *VDIDescriptor) Load(c *Connection) error {
	var vdi xenAPI.VDIRef

	hasVDIName := false
	hasVDIUUID := false

	if this.Name != "" {
		vdis, err := c.client.VDI.GetByNameLabel(c.session, this.Name)
		if err != nil {
			return err
		}

		if len(vdis) == 0 {
			return fmt.Errorf("VDI %q not found!", this.Name)
		}

		hasVDIName = true
		vdi = vdis[0]
	}

	if !hasVDIName {
		if this.UUID != "" {
			_vdi, err := c.client.VDI.GetByUUID(c.session, this.UUID)
			if err != nil {
				return err
			}
			hasVDIUUID = true
			vdi = _vdi
		}
	}

	if !hasVDIName && !hasVDIUUID {
		return fmt.Errorf("Either name_label or UUID should be specified!")
	}

	this.VDIRef = vdi

	return this.Query(c)
}

func (this *VDIDescriptor) Query(c *Connection) error {
	vdi, err := c.client.VDI.GetRecord(c.session, this.VDIRef)
	if err != nil {
		return err
	}

	this.UUID = vdi.UUID
	this.Name = vdi.NameLabel
	this.IsReadOnly = vdi.ReadOnly
	this.IsShared = vdi.Sharable
	this.Size = vdi.VirtualSize

	sr := &SRDescriptor{
		SRRef: vdi.SR,
	}

	if err = sr.Query(c); err != nil {
		return err
	}

	this.SR = sr

	return nil
}

/*func (this *VBDDescriptor) Load(c *Connection) error {
	var vbd xenAPI.VBDRef

	if this.UUID != "" {
		_vbd, err := c.client.VBD.GetByUUID(c.session, this.UUID)
		if err != nil {
			return err
		}
		vbd = _vbd
	} else {
		return fmt.Errorf("%q should be specified!", vbdSchemaUUID)
	}

	this.VBDRef = vbd

	return this.Query(c)
}*/

func (this *VBDDescriptor) Query(c *Connection) error {

	log.Println("[DEBUG] Query VBD")

	vbd, err := c.client.VBD.GetRecord(c.session, this.VBDRef)
	if err != nil {
		return err
	}

	this.UUID = vbd.UUID
	this.Type = vbd.Type
	this.Device = vbd.Device
	this.UserDevice = vbd.Userdevice
	this.Bootable = vbd.Bootable
	this.Mode = vbd.Mode
	this.OtherConfig = vbd.OtherConfig

	isTemplateDevice := false

	if val, ok := this.OtherConfig[vbdSchemaTemplateDevice]; ok {
		log.Printf("[DEBUG] Got from OtherConfig %s\n", val)
		if parsed, err := strconv.ParseBool(val); err == nil {
			log.Printf("[DEBUG] Parsed from OtherConfig %t\n", parsed)
			isTemplateDevice = parsed
		} else {
			log.Printf("[ERROR] Cannot parse %s as boolean value; got %s", vbdSchemaTemplateDevice, val)
		}
	}

	this.IsTemplateDevice = isTemplateDevice

	vm := &VMDescriptor{
		VMRef: vbd.VM,
	}

	if err := vm.Query(c); err != nil {
		return err
	}

	this.VM = vm

	vdi := &VDIDescriptor{
		VDIRef: vbd.VDI,
	}

	if err := vdi.Query(c); err != nil {
		return err
	}

	this.VDI = vdi

	return nil
}

func (this *VBDDescriptor) Commit(c *Connection) (err error) {

	if err = c.client.VBD.SetBootable(c.session, this.VBDRef, this.Bootable); err != nil {
		return err
	}

	if err = c.client.VBD.SetMode(c.session, this.VBDRef, this.Mode); err != nil {
		return err
	}

	this.OtherConfig[vbdSchemaTemplateDevice] = strconv.FormatBool(this.IsTemplateDevice)

	if err = c.client.VBD.SetOtherConfig(c.session, this.VBDRef, this.OtherConfig); err != nil {
		return err
	}

	log.Println("[DEBUG] VBD Commited")

	return nil
}

func (this *PIFDescriptor) Load(c *Connection) error {
	var pif xenAPI.PIFRef

	if this.UUID != "" {
		_vbd, err := c.client.PIF.GetByUUID(c.session, this.UUID)
		if err != nil {
			return err
		}
		pif = _vbd
	} else {
		return fmt.Errorf("%q should be specified!", pifSchemaUUID)
	}

	this.PIFRef = pif

	return this.Query(c)
}

func (this *PIFDescriptor) Query(c *Connection) error {
	pif, err := c.client.PIF.GetRecord(c.session, this.PIFRef)
	if err != nil {
		return err
	}

	this.UUID = pif.UUID

	return nil
}

func (this *VLANDescriptor) Load(c *Connection) error {
	var vlan xenAPI.VLANRef

	if this.UUID != "" {
		_vbd, err := c.client.VLAN.GetByUUID(c.session, this.UUID)
		if err != nil {
			return err
		}
		vlan = _vbd
	} else {
		return fmt.Errorf("%q should be specified!", vlanSchemaTag)
	}

	this.VLANRef = vlan

	return this.Query(c)
}

func (this *VLANDescriptor) Query(c *Connection) error {
	vlan, err := c.client.VLAN.GetRecord(c.session, this.VLANRef)
	if err != nil {
		return err
	}

	this.UUID = vlan.UUID
	this.Tag = vlan.Tag
	this.OtherConfig = vlan.OtherConfig

	if vlan.TaggedPIF != "" {
		var taggedPif = PIFDescriptor{
			PIFRef: vlan.TaggedPIF,
		}

		err := taggedPif.Query(c)
		if err != nil {
			return err
		}
		this.TaggedPIF = taggedPif
	}

	if vlan.UntaggedPIF != "" {
		var untaggedPif = PIFDescriptor{
			PIFRef: vlan.UntaggedPIF,
		}

		err := untaggedPif.Query(c)
		if err != nil {
			return err
		}
		this.UntaggedPIF = untaggedPif
	}

	return nil
}
