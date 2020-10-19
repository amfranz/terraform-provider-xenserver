package xenserver

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
)

func dataSourceXenServerNetwork() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceXenServerNetworkRead,

		Schema: map[string]*schema.Schema{
			"name_label": &schema.Schema{
				Type:        schema.TypeString,
				Description: "The human readable name of the Network",
				Required:    true,
			},
		},
	}
}

func dataSourceXenServerNetworkRead(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*Connection)

	nameLabel, nameLabelOk := d.GetOk("name_label")

	if !nameLabelOk {
		return fmt.Errorf("name_label must be provided")
	}

	if srs, err := c.client.Network.GetByNameLabel(c.session, nameLabel.(string)); err == nil {
		found := false
		for _, sr := range srs {
			record, err := c.client.Network.GetRecord(c.session, sr)
			if err != nil {
				break
			}

			d.SetId(record.UUID)

			found = true
			break
		}

		if !found {
			return fmt.Errorf("Matching Network not found")
		}
	}

	return nil
}
