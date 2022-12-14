package common

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"

	"tenant-native-terraform-generator/duplosdk"

	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/zclconf/go-cty/cty"
)

type Provider struct {
}

func (p *Provider) Generate(config *Config, client *duplosdk.Client) {
	log.Println("[TRACE] <====== Provider TF generation started. =====>")
	log.Printf("Config - %s", fmt.Sprintf("%#v", config))
	// create new empty hcl file object
	hclFile := hclwrite.NewEmptyFile()

	// create new file on system
	tenantProject := filepath.Join(config.TFCodePath, config.TenantProject, "providers.tf")
	tenantProjectFile, err := os.Create(tenantProject)
	if err != nil {
		fmt.Println(err)
		return
	}

	// initialize the body of the new file object
	rootBody := hclFile.Body()

	// Add duplo terraform block
	tfBlock := rootBody.AppendNewBlock("terraform",
		nil)
	tfBlockBody := tfBlock.Body()
	tfVersion := GetEnv("tf_version", "0.14.11")
	tfBlockBody.SetAttributeValue("required_version",
		cty.StringVal(">= "+tfVersion))

	reqProvsBlock := tfBlockBody.AppendNewBlock("required_providers",
		nil)
	reqProvsBlockBody := reqProvsBlock.Body()

	reqProvsBlockBody.SetAttributeValue("aws",
		cty.ObjectVal(map[string]cty.Value{
			"source":  cty.StringVal("hashicorp/aws"),
			"version": cty.StringVal("~> " + config.AwsProviderVersion),
		}))
	// reqProvsBlockBody.SetAttributeValue("tls",
	// 	cty.ObjectVal(map[string]cty.Value{
	// 		"source":  cty.StringVal("hashicorp/tls"),
	// 		"version": cty.StringVal("~> 4.0.3"),
	// 	}))

	awsProvider := rootBody.AppendNewBlock("provider",
		[]string{"aws"})
	awsProviderBody := awsProvider.Body()
	awsProviderBody.SetAttributeTraversal("region", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "var",
		},
		hcl.TraverseAttr{
			Name: "region",
		},
	})
	fmt.Printf("%s", hclFile.Bytes())
	_, err = tenantProjectFile.Write(hclFile.Bytes())
	if err != nil {
		fmt.Println(err)
		return
	}
	reqProvsBlockBody.SetAttributeValue("random",
		cty.ObjectVal(map[string]cty.Value{
			"source":  cty.StringVal("hashicorp/random"),
			"version": cty.StringVal("~> 3.3.2"),
		}))
	randomProvider := rootBody.AppendNewBlock("provider",
		[]string{"random"})

	randomProviderBody := randomProvider.Body()
	randomProviderBody.AppendNewline()

	log.Println("[TRACE] <====== Provider TF generation done. =====>")
}
