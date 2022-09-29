package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tenant-native-terraform-generator/duplosdk"

	"tenant-native-terraform-generator/tf-generator/common"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

const (
	KMS_NAME                     string = "name"
	KMS_DESCRIPTION              string = "description"
	KMS_CUSTOMER_MASTER_KEY_SPEC string = "customer_master_key_spec"
	KMS_ENABLE_KEY_ROTATION      string = "enable_key_rotation"
	KMS_POLICY                   string = "policy"
	KMS_KEY_USAGE                string = "key_usage"
	KMS_TARGET_KEY_ID            string = "target_key_id"
)

const AWS_KMS_KEY = "aws_kms_key"
const AWS_KMS_ALIAS = "aws_kms_alias"
const TENANT_KMS = "tenant_kms"
const TENANT_KMS_FILE_NAME = "tenant-kms"

type TenantKMS struct {
}

func (tenantKMS *TenantKMS) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	resourceName := TENANT_KMS

	duplo, clientErr := client.TenantGetTenantKmsKey(config.TenantId)
	if clientErr != nil {
		fmt.Println(clientErr)
		return nil, clientErr
	}
	kmsClient := kms.NewFromConfig(config.AwsClientConfig)
	describeKeyOutput, err := kmsClient.DescribeKey(context.TODO(), &kms.DescribeKeyInput{KeyId: &duplo.KeyID})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defaultPolicy := "default"
	getKeyPolicyOutput, err := kmsClient.GetKeyPolicy(context.TODO(), &kms.GetKeyPolicyInput{KeyId: &duplo.KeyID, PolicyName: &defaultPolicy})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if describeKeyOutput != nil {
		hclFile := hclwrite.NewEmptyFile()
		path := filepath.Join(workingDir, TENANT_KMS_FILE_NAME+".tf")
		tfFile, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		rootBody := hclFile.Body()

		kmsBlock := rootBody.AppendNewBlock("resource",
			[]string{AWS_KMS_KEY,
				resourceName})
		kmsBody := kmsBlock.Body()

		if describeKeyOutput.KeyMetadata != nil && describeKeyOutput.KeyMetadata.Description != nil {
			kmsBody.SetAttributeTraversal(KMS_DESCRIPTION, hcl.Traversal{
				hcl.TraverseRoot{
					Name: "local",
				},
				hcl.TraverseAttr{
					Name: "tenant_prefix",
				},
			})
		}
		if describeKeyOutput.KeyMetadata != nil && len(describeKeyOutput.KeyMetadata.KeyUsage) > 0 {
			kmsBody.SetAttributeValue(KMS_KEY_USAGE,
				cty.StringVal(string(describeKeyOutput.KeyMetadata.KeyUsage)))
		}
		if describeKeyOutput.KeyMetadata != nil && len(describeKeyOutput.KeyMetadata.KeySpec) > 0 {
			kmsBody.SetAttributeValue(KMS_CUSTOMER_MASTER_KEY_SPEC,
				cty.StringVal(string(describeKeyOutput.KeyMetadata.KeySpec)))
		}
		kmsBody.SetAttributeValue(KMS_ENABLE_KEY_ROTATION,
			cty.BoolVal(true))

		if getKeyPolicyOutput != nil && getKeyPolicyOutput.Policy != nil {
			iamClient := iam.NewFromConfig(config.AwsClientConfig)
			iamRoleName := "duploservices-" + config.TenantName
			// Get Role
			getRoleOutput, err := iamClient.GetRole(context.TODO(), &iam.GetRoleInput{RoleName: &iamRoleName})
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			var policyMap interface{}
			err = json.Unmarshal([]byte(*getKeyPolicyOutput.Policy), &policyMap)
			if err != nil {
				panic(err)
			}
			policyMapStr, err := duplosdk.JSONMarshal(policyMap)
			if err != nil {
				panic(err)
			}
			replaceStr := strings.Join([]string{"${" +
				AWS_IAM_ROLE,
				TENANT_IAM, "arn}",
			}, ".")
			accountIdStr := "${local.account_id}"
			policyMapStr = strings.Replace(policyMapStr, *getRoleOutput.Role.Arn, replaceStr, -1)
			policyMapStr = strings.Replace(policyMapStr, config.AccountID, accountIdStr, -1)

			kmsBody.SetAttributeTraversal(KMS_POLICY, hcl.Traversal{
				hcl.TraverseRoot{
					Name: "jsonencode(" + policyMapStr + ")",
				},
			})
		}

		kmsAliasBlock := rootBody.AppendNewBlock("resource",
			[]string{AWS_KMS_ALIAS,
				resourceName})
		kmsAliasBody := kmsAliasBlock.Body()

		alias := "alias/${local.tenant_prefix}"
		aliasTokens := hclwrite.Tokens{
			{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
			{Type: hclsyntax.TokenIdent, Bytes: []byte(alias)},
			{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
		}
		kmsAliasBody.SetAttributeRaw(KMS_NAME, aliasTokens)

		kmsAliasBody.SetAttributeTraversal(KMS_TARGET_KEY_ID, hcl.Traversal{
			hcl.TraverseRoot{
				Name: "aws_kms_key." + resourceName,
			},
			hcl.TraverseAttr{
				Name: "key_id",
			},
		})

		if config.GenerateTfState {
			importConfigs = append(importConfigs, common.ImportConfig{
				ResourceAddress: strings.Join([]string{
					AWS_KMS_KEY,
					resourceName,
				}, "."),
				ResourceId: *describeKeyOutput.KeyMetadata.KeyId,
				WorkingDir: workingDir,
			}, common.ImportConfig{
				ResourceAddress: strings.Join([]string{
					AWS_KMS_ALIAS,
					resourceName,
				}, "."),
				ResourceId: "alias/" + duplo.KeyName,
				WorkingDir: workingDir,
			})
			tfContext.ImportConfigs = importConfigs
		}
		_, err = tfFile.Write(hclFile.Bytes())
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	}

	return &tfContext, nil
}
