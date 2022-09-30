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

	localsBlockBody.SetAttributeTraversal("account_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "data.aws_caller_identity.current",
		},
		hcl.TraverseAttr{
			Name: "account_id",
		},
	})
	localsBlockBody.SetAttributeTraversal("region", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "var",
		},
		hcl.TraverseAttr{
			Name: "region",
		},
	})
	localsBlockBody.SetAttributeTraversal("vpc_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "var",
		},
		hcl.TraverseAttr{
			Name: "vpc_id",
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

	tenantPrefix := "duploservices-${var.tenant_name}"
	tenantPrefixTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(tenantPrefix)},
		{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
	}
	localsBlockBody.SetAttributeRaw("tenant_prefix", tenantPrefixTokens)

	tenantIAMRole := "duploservices-${var.tenant_name}"
	tenantIAMRoleTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(tenantIAMRole)},
		{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
	}
	localsBlockBody.SetAttributeRaw("tenant_iam_role_name", tenantIAMRoleTokens)

	tenantSG := "duploservices-${var.tenant_name}"
	tenantSGTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(tenantSG)},
		{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
	}
	localsBlockBody.SetAttributeRaw("tenant_sg_name", tenantSGTokens)

	tenantLBSG := "duploservices-${var.tenant_name}-lb"
	tenantLBSGTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(tenantLBSG)},
		{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
	}
	localsBlockBody.SetAttributeRaw("tenant_lb_sg_name", tenantLBSGTokens)

	tenantALBSG := "duploservices-${var.tenant_name}-alb"
	tenantALBSGTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(tenantALBSG)},
		{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
	}
	localsBlockBody.SetAttributeRaw("tenant_alb_sg_name", tenantALBSGTokens)

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
