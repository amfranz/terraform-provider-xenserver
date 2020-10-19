package xenserver

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

// Provider ...
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"url": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["url"],
			},

			"username": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["username"],
			},

			"password": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["password"],
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"xenserver_pif":     dataSourceXenServerPif(),
			"xenserver_pifs":    dataSourceXenServerPifs(),
			"xenserver_sr":      dataSourceXenServerSR(),
			"xenserver_network": dataSourceXenServerNetwork(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"xenserver_vm":      resourceVM(),
			"xenserver_vdi":     resourceVDI(),
			"xenserver_network": resourceNetwork(),
		},

		ConfigureFunc: providerConfigure,
	}
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"url": "The URL to the XenAPI endpoint, typically \"https://<XenServer Management IP>\"",

		"username": "The username to use to authenticate to XenServer",

		"password": "The password to use to authenticate to XenServer",
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	config := Config{
		URL:      d.Get("url").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
	}

	return config.NewConnection()
}
