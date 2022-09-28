package tenant

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"tenant-native-terraform-generator/duplosdk"
	"tenant-native-terraform-generator/tf-generator/common"

	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type TenantMain struct {
}

func (tm *TenantMain) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)

	log.Println("[TRACE] <====== Tenant main TF generation started. =====>")

	//1. ==========================================================================================
	// Generate locals
	hclFile := hclwrite.NewEmptyFile()

	// create new file on system
	path := filepath.Join(workingDir, "main.tf")
	tfFile, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// initialize the body of the new file object
	rootBody := hclFile.Body()

	awsCallerIdBlock := rootBody.AppendNewBlock("data",
		[]string{"aws_caller_identity",
			"current"})
	awsCallerIdBody := awsCallerIdBlock.Body()
	awsCallerIdBody.Clear()
	rootBody.AppendNewline()

	regionBlock := rootBody.AppendNewBlock("data",
		[]string{"aws_region",
			"current"})
	regionBody := regionBlock.Body()
	regionBody.Clear()
	rootBody.AppendNewline()

	localsBlock := rootBody.AppendNewBlock("locals",
		nil)
	localsBlockBody := localsBlock.Body()

	localsBlockBody.SetAttributeTraversal("region", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "var",
		},
		hcl.TraverseAttr{
			Name: "region",
		},
	})
	localsBlockBody.SetAttributeTraversal("tenant_name", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "var",
		},
		hcl.TraverseAttr{
			Name: "tenant_name",
		},
	})

	tenantIAMRole := "duploservices-${var.tenant_name}"
	dnsPrefixTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(tenantIAMRole)},
		{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
	}
	localsBlockBody.SetAttributeRaw("tenant_iam_role_name", dnsPrefixTokens)
	rootBody.AppendNewline()

	fmt.Printf("%s", hclFile.Bytes())
	_, err = tfFile.Write(hclFile.Bytes())
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	log.Println("[TRACE] <====== Tenant main TF generation done. =====>")
	return nil, nil
}
