package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"tenant-native-terraform-generator/duplosdk"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"tenant-native-terraform-generator/tf-generator/common"
)

const (
	ROLE_NAME          string = "name"
	PATH               string = "path"
	ROLE               string = "role"
	POLICY_ARN         string = "policy_arn"
	ROLE_DESCRIPTION   string = "description"
	ASSUME_ROLE_POLICY string = "assume_role_policy"
	INLINE_POLICY      string = "inline_policy"
	POLICY             string = "policy"
)

const TENANT_IAM = "tenant_iam"
const AWS_IAM_ROLE = "aws_iam_role"
const AWS_IAM_POLICY = "aws_iam_policy"
const AWS_IAM_ROLE_POLICY_ATTACHMENT = "aws_iam_role_policy_attachment"
const TENANT_IAM_FILE_NAME_PREFIX = "tenant-iam"

type TenantIAM struct {
}

func (tenantIAM *TenantIAM) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	tfContext := common.TFContext{}

	importConfigs := []common.ImportConfig{}
	iamRoleName := "duploservices-" + config.TenantName

	iamClient := iam.NewFromConfig(config.AwsClientConfig)

	// Get Role
	getRoleOutput, err := iamClient.GetRole(context.TODO(), &iam.GetRoleInput{RoleName: &iamRoleName})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	log.Println("[TRACE] <====== Tenant IAM Role TF generation started. =====>")
	log.Printf("Reading IAM role from AWS, Role - %s", iamRoleName)

	if getRoleOutput != nil && getRoleOutput.Role != nil {
		iamRole := getRoleOutput.Role
		resourceName := TENANT_IAM

		hclFile := hclwrite.NewEmptyFile()

		path := filepath.Join(workingDir, TENANT_IAM_FILE_NAME_PREFIX+".tf")
		tfFile, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		rootBody := hclFile.Body()

		// Add aws_iam_role resource
		iamRoleBlock := rootBody.AppendNewBlock("resource",
			[]string{AWS_IAM_ROLE,
				resourceName})
		iamRoleBody := iamRoleBlock.Body()

		iamRoleBody.SetAttributeTraversal(ROLE_NAME, hcl.Traversal{
			hcl.TraverseRoot{
				Name: "local",
			},
			hcl.TraverseAttr{
				Name: "tenant_iam_role_name",
			},
		})
		// iamRoleBody.SetAttributeValue(NAME,
		// 	cty.StringVal(*iamRole.RoleName))
		decodedAssumeRolePolicyDocument, err := url.QueryUnescape(*iamRole.AssumeRolePolicyDocument)
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		// Add 'assume_role_policy'
		if len(decodedAssumeRolePolicyDocument) > 0 {

			decodedAssumeRolePolicyDocument = strings.Replace(decodedAssumeRolePolicyDocument, iamRoleName, "${local.tenant_iam_role_name}", -1)
			decodedAssumeRolePolicyDocument = strings.Replace(decodedAssumeRolePolicyDocument, config.TenantName, "${local.tenant_name}", -1)
			accountIdStr := "${local.account_id}"
			decodedAssumeRolePolicyDocument = strings.Replace(decodedAssumeRolePolicyDocument, config.AccountID, accountIdStr, -1)
			assumeRolePolicyDocumentMap := make(map[string]interface{})
			if err := json.Unmarshal([]byte(decodedAssumeRolePolicyDocument), &assumeRolePolicyDocumentMap); err != nil {
				log.Fatal(err)
				return nil, err
			}
			assumeRolePolicyDocumentStr, err := duplosdk.JSONMarshal(assumeRolePolicyDocumentMap)
			if err != nil {
				panic(err)
			}
			iamRoleBody.SetAttributeTraversal(ASSUME_ROLE_POLICY, hcl.Traversal{
				hcl.TraverseRoot{
					Name: "jsonencode(" + assumeRolePolicyDocumentStr + ")",
				},
			})
		}
		// Add 'inline_policy'
		listRolePoliciesOutput, err := iamClient.ListRolePolicies(context.TODO(), &iam.ListRolePoliciesInput{RoleName: &iamRoleName})
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		// Add 'inline_policy'
		if listRolePoliciesOutput != nil && listRolePoliciesOutput.PolicyNames != nil {
			for _, policyName := range listRolePoliciesOutput.PolicyNames {
				getRolePolicyOutput, err := iamClient.GetRolePolicy(context.TODO(), &iam.GetRolePolicyInput{
					RoleName:   &iamRoleName,
					PolicyName: &policyName,
				})
				if err != nil {
					fmt.Println(err)
					return nil, err
				}

				inlinePolicyBlock := iamRoleBody.AppendNewBlock("inline_policy",
					nil)
				inlinePolicyBody := inlinePolicyBlock.Body()
				inlinePolicyBody.SetAttributeValue(ROLE_NAME,
					cty.StringVal(policyName))
				if getRolePolicyOutput.PolicyDocument != nil {
					decodedInlinePolicyDocument, err := url.QueryUnescape(*getRolePolicyOutput.PolicyDocument)
					decodedInlinePolicyDocument = strings.Replace(decodedInlinePolicyDocument, iamRoleName, "${local.tenant_iam_role_name}", -1)
					decodedInlinePolicyDocument = strings.Replace(decodedInlinePolicyDocument, config.TenantName, "${local.tenant_name}", -1)
					accountIdStr := "${local.account_id}"
					decodedInlinePolicyDocument = strings.Replace(decodedInlinePolicyDocument, config.AccountID, accountIdStr, -1)
					if err != nil {
						log.Fatal(err)
						return nil, err
					}
					inlineRolePolicyDocumentMap := make(map[string]interface{})
					if err := json.Unmarshal([]byte(decodedInlinePolicyDocument), &inlineRolePolicyDocumentMap); err != nil {
						log.Fatal(err)
						return nil, err
					}
					inlineRolePolicyDocumentStr, err := duplosdk.JSONMarshal(inlineRolePolicyDocumentMap)
					if err != nil {
						panic(err)
					}
					inlinePolicyBody.SetAttributeTraversal(POLICY, hcl.Traversal{
						hcl.TraverseRoot{
							Name: "jsonencode(" + inlineRolePolicyDocumentStr + ")",
						},
					})
				}

			}
		}

		listAttachedRolePoliciesOutput, err := iamClient.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{RoleName: &iamRoleName})
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		// Add 'aws_iam_policy' for managed policies
		if listAttachedRolePoliciesOutput != nil && len(listAttachedRolePoliciesOutput.AttachedPolicies) > 0 {
			for _, policy := range listAttachedRolePoliciesOutput.AttachedPolicies {
				getPolicyOutput, err := iamClient.GetPolicy(context.TODO(), &iam.GetPolicyInput{
					PolicyArn: policy.PolicyArn,
				})
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				policyDetails := *getPolicyOutput.Policy
				policyResourceName := common.GetResourceName(*policyDetails.PolicyName)
				rootBody.AppendNewline()
				iamPolicyBlock := rootBody.AppendNewBlock("resource",
					[]string{AWS_IAM_POLICY,
						policyResourceName})
				iamPolicyBody := iamPolicyBlock.Body()
				iamPolicyBody.SetAttributeValue(ROLE_NAME,
					cty.StringVal(*policyDetails.PolicyName))
				if policyDetails.Path != nil {
					iamPolicyBody.SetAttributeValue(PATH,
						cty.StringVal(*policyDetails.Path))
				}
				if policyDetails.Description != nil {
					iamPolicyBody.SetAttributeValue(ROLE_DESCRIPTION,
						cty.StringVal(*policyDetails.Description))
				}
				getPolicyVersionOutput, err := iamClient.GetPolicyVersion(context.TODO(), &iam.GetPolicyVersionInput{
					PolicyArn: policy.PolicyArn,
					VersionId: getPolicyOutput.Policy.DefaultVersionId,
				})
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				if getPolicyVersionOutput != nil && getPolicyVersionOutput.PolicyVersion.Document != nil {
					decodedManagedPolicyDocument, err := url.QueryUnescape(*getPolicyVersionOutput.PolicyVersion.Document)
					decodedManagedPolicyDocument = strings.Replace(decodedManagedPolicyDocument, iamRoleName, "${local.tenant_iam_role_name}", -1)
					decodedManagedPolicyDocument = strings.Replace(decodedManagedPolicyDocument, config.TenantName, "${local.tenant_name}", -1)
					accountIdStr := "${local.account_id}"
					decodedManagedPolicyDocument = strings.Replace(decodedManagedPolicyDocument, config.AccountID, accountIdStr, -1)
					if err != nil {
						log.Fatal(err)
						return nil, err
					}
					managedRolePolicyDocumentMap := make(map[string]interface{})
					if err := json.Unmarshal([]byte(decodedManagedPolicyDocument), &managedRolePolicyDocumentMap); err != nil {
						log.Fatal(err)
						return nil, err
					}
					managedRolePolicyDocumentStr, err := duplosdk.JSONMarshal(managedRolePolicyDocumentMap)
					if err != nil {
						panic(err)
					}
					iamPolicyBody.SetAttributeTraversal(POLICY, hcl.Traversal{
						hcl.TraverseRoot{
							Name: "jsonencode(" + managedRolePolicyDocumentStr + ")",
						},
					})
				}
				// Add 'aws_iam_role_policy_attachment' resource
				rootBody.AppendNewline()
				iamPolicyAttachBlock := rootBody.AppendNewBlock("resource",
					[]string{AWS_IAM_ROLE_POLICY_ATTACHMENT,
						common.GetResourceName(*policyDetails.PolicyName) + "_attach"})
				iamPolicyAtatchBody := iamPolicyAttachBlock.Body()

				iamPolicyAtatchBody.SetAttributeTraversal(ROLE, hcl.Traversal{
					hcl.TraverseRoot{
						Name: strings.Join([]string{
							AWS_IAM_ROLE,
							resourceName,
						}, "."),
					},
					hcl.TraverseAttr{
						Name: "name",
					},
				})
				iamPolicyAtatchBody.SetAttributeTraversal(POLICY_ARN, hcl.Traversal{
					hcl.TraverseRoot{
						Name: strings.Join([]string{
							AWS_IAM_POLICY,
							policyResourceName,
						}, "."),
					},
					hcl.TraverseAttr{
						Name: "arn",
					},
				})
				if config.GenerateTfState {
					importConfigs = append(importConfigs, common.ImportConfig{
						ResourceAddress: strings.Join([]string{
							AWS_IAM_POLICY,
							policyResourceName,
						}, "."),
						ResourceId: *policy.PolicyArn,
						WorkingDir: workingDir,
					}, common.ImportConfig{
						ResourceAddress: strings.Join([]string{
							AWS_IAM_ROLE_POLICY_ATTACHMENT,
							policyResourceName + "_attach",
						}, "."),
						ResourceId: iamRoleName + "/" + *policy.PolicyArn,
						WorkingDir: workingDir,
					})
					tfContext.ImportConfigs = importConfigs
				}
			}
		}
		// Import all created resources.
		if config.GenerateTfState {
			importConfigs = append(importConfigs, common.ImportConfig{
				ResourceAddress: strings.Join([]string{
					AWS_IAM_ROLE,
					resourceName,
				}, "."),
				ResourceId: iamRoleName,
				WorkingDir: workingDir,
			})
			tfContext.ImportConfigs = importConfigs
		}
		log.Println("[TRACE] <====== Tenant IAM Role TF generation done. =====>")
		_, err = tfFile.Write(hclFile.Bytes())
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	}
	return &tfContext, nil
}
