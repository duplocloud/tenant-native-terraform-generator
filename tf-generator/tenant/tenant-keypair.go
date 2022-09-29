package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tenant-native-terraform-generator/duplosdk"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"tenant-native-terraform-generator/tf-generator/common"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

const (
	KEYPAIR_ALGORITHM  string = "algorithm"
	KEYPAIR_RSA_BITS   string = "rsa_bits"
	KEYPAIR_KEY_NAME   string = "key_name"
	KEYPAIR_PUBLIC_KEY string = "public_key"
	KEYPAIR_TAGS       string = "tags"
)

const AWS_KEY_PAIR = "aws_key_pair"
const TENANT_KEYPAIR = "tenant_keypair"
const TLS_PRIVATE_KEY = "tls_private_key"
const TENANT_KEYPAIR_FILE_NAME = "tenant-keypair"

type TenantKeyPair struct {
}

func (tenantKeyPair *TenantKeyPair) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	keyPairName := "duploservices-" + config.TenantName
	includePublicKey := true
	ec2Client := ec2.NewFromConfig(config.AwsClientConfig)
	describeKeyPairsOutput, err := ec2Client.DescribeKeyPairs(context.TODO(), &ec2.DescribeKeyPairsInput{
		KeyNames:         []string{keyPairName},
		IncludePublicKey: &includePublicKey,
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	b, err := json.Marshal(describeKeyPairsOutput)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("||==================================================================||")
	fmt.Println(string(b))
	fmt.Println("||==================================================================||")
	resourceName := TENANT_KEYPAIR
	hclFile := hclwrite.NewEmptyFile()
	path := filepath.Join(workingDir, TENANT_KEYPAIR_FILE_NAME+".tf")
	tfFile, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	inputVars := generateTenantKeyPairVars(describeKeyPairsOutput)
	tfContext.InputVars = append(tfContext.InputVars, inputVars...)

	rootBody := hclFile.Body()

	// // Add tls_private_key resource
	// tlsBlock := rootBody.AppendNewBlock("resource",
	// 	[]string{TLS_PRIVATE_KEY,
	// 		resourceName})
	// tlsBody := tlsBlock.Body()
	// tlsBody.SetAttributeValue(KEYPAIR_ALGORITHM,
	// 	cty.StringVal("RSA"))
	// tlsBody.SetAttributeValue(KEYPAIR_RSA_BITS,
	// 	cty.NumberIntVal(int64(4096)))

	// Add aws_key_pair resource
	kpBlock := rootBody.AppendNewBlock("resource",
		[]string{AWS_KEY_PAIR,
			resourceName})
	kpBody := kpBlock.Body()
	kpBody.SetAttributeTraversal(KEYPAIR_KEY_NAME, hcl.Traversal{
		hcl.TraverseRoot{
			Name: "local",
		},
		hcl.TraverseAttr{
			Name: "tenant_prefix",
		},
	})
	kpBody.SetAttributeTraversal(KEYPAIR_PUBLIC_KEY, hcl.Traversal{
		hcl.TraverseRoot{
			Name: "var",
		},
		hcl.TraverseAttr{
			Name: "tenant_key_pair_public_key",
		},
	})
	if describeKeyPairsOutput != nil && len(describeKeyPairsOutput.KeyPairs) > 0 && len(describeKeyPairsOutput.KeyPairs[0].Tags) > 0 {
		newMap := make(map[string]cty.Value)
		for _, tag := range describeKeyPairsOutput.KeyPairs[0].Tags {
			newMap[*tag.Key] = cty.StringVal(*tag.Value)
		}
		kpBody.SetAttributeValue(KEYPAIR_TAGS, cty.MapVal(newMap))
	}
	if config.GenerateTfState {
		importConfigs = append(importConfigs, common.ImportConfig{
			ResourceAddress: strings.Join([]string{
				AWS_KEY_PAIR,
				resourceName,
			}, "."),
			ResourceId: keyPairName,
			WorkingDir: workingDir,
		})
		tfContext.ImportConfigs = importConfigs
	}
	_, err = tfFile.Write(hclFile.Bytes())
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return &tfContext, nil
}

func generateTenantKeyPairVars(keypair *ec2.DescribeKeyPairsOutput) []common.VarConfig {
	varConfigs := make(map[string]common.VarConfig)

	publicKeyVar := common.VarConfig{
		Name:       "tenant_key_pair_public_key",
		DefaultVal: *keypair.KeyPairs[0].PublicKey,
		TypeVal:    "string",
	}
	varConfigs["tenant_key_pair_public_key"] = publicKeyVar

	vars := make([]common.VarConfig, len(varConfigs))
	for _, v := range varConfigs {
		vars = append(vars, v)
	}
	return vars
}
