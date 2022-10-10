package tenant

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"tenant-native-terraform-generator/duplosdk"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"tenant-native-terraform-generator/tf-generator/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	SG_NAME             string = "name"
	SG_VPC_ID           string = "vpc_id"
	SG_FROM_PORT        string = "from_port"
	SG_TO_PORT          string = "to_port"
	SG_DESCRIPTION      string = "description"
	SG_PROTOCOL         string = "protocol"
	SG_CIDR_BLOCKS      string = "cidr_blocks"
	SG_IPV6_CIDR_BLOCKS string = "ipv6_cidr_blocks"
	SG_TAGS             string = "tags"
	SG_PREFIX_LIST_IDS  string = "prefix_list_ids"
	SG_SECURITY_GROUPS  string = "security_groups"
	SG_SELF             string = "self"
	SG_INGRESS          string = "ingress"
	SG_EGRESS           string = "egress"
)

const TENANT_SG = "tenant_sg"
const AWS_SECURITY_GROUP = "aws_security_group"
const SG_FILE_NAME_PREFIX = "tenant-sg"

type TenantSG struct {
}

func (tenantSG *TenantSG) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	ec2Client := ec2.NewFromConfig(config.AwsClientConfig)
	filteName := "group-name"
	describeSecurityGroupsOutput, err := ec2Client.DescribeSecurityGroups(context.TODO(), &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name: &filteName,
				Values: []string{
					"duploservices-" + config.TenantName, "duploservices-" + config.TenantName + "-lb", "duploservices-" + config.TenantName + "-alb",
				},
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if describeSecurityGroupsOutput != nil && len(describeSecurityGroupsOutput.SecurityGroups) > 0 {
		hclFile := hclwrite.NewEmptyFile()
		path := filepath.Join(workingDir, SG_FILE_NAME_PREFIX+".tf")
		tfFile, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		// b, err := json.Marshal(describeSecurityGroupsOutput)
		// if err != nil {
		// 	fmt.Println(err)
		// }
		// fmt.Println("||==================================================================||")
		// fmt.Println(string(b))
		// fmt.Println("||==================================================================||")
		rootBody := hclFile.Body()
		for _, sg := range describeSecurityGroupsOutput.SecurityGroups {
			log.Printf("[TRACE] Terraform config generation started for aws security group (%s).", *sg.GroupName)
			resourceName := common.GetResourceName(*sg.GroupName)
			sgBlock := rootBody.AppendNewBlock("resource",
				[]string{AWS_SECURITY_GROUP,
					resourceName})
			sgBody := sgBlock.Body()
			// sgBody.SetAttributeValue(SG_NAME,
			// 	cty.StringVal(*sg.GroupName))
			if "duploservices-"+config.TenantName == *sg.GroupName {
				sgBody.SetAttributeTraversal(SG_NAME, hcl.Traversal{
					hcl.TraverseRoot{
						Name: "local",
					},
					hcl.TraverseAttr{
						Name: "tenant_sg_name",
					},
				})
			}
			if "duploservices-"+config.TenantName+"-lb" == *sg.GroupName {
				sgBody.SetAttributeTraversal(SG_NAME, hcl.Traversal{
					hcl.TraverseRoot{
						Name: "local",
					},
					hcl.TraverseAttr{
						Name: "tenant_lb_sg_name",
					},
				})
			}
			if "duploservices-"+config.TenantName+"-alb" == *sg.GroupName {
				sgBody.SetAttributeTraversal(SG_NAME, hcl.Traversal{
					hcl.TraverseRoot{
						Name: "local",
					},
					hcl.TraverseAttr{
						Name: "tenant_alb_sg_name",
					},
				})
			}
			if sg.Description != nil && len(*sg.Description) > 0 {
				// desc := *sg.Description
				// desc = strings.Replace(desc, config.TenantName, "${local.tenant_name}", -1)
				sgBody.SetAttributeValue(SG_DESCRIPTION,
					cty.StringVal(*sg.Description))
			}
			sgBody.SetAttributeTraversal(SG_VPC_ID, hcl.Traversal{
				hcl.TraverseRoot{
					Name: "local",
				},
				hcl.TraverseAttr{
					Name: SG_VPC_ID,
				},
			})
			if len(sg.IpPermissions) > 0 {
				for _, ingress := range sg.IpPermissions {
					ingressBlock := sgBody.AppendNewBlock(SG_INGRESS,
						nil)
					ingressBody := ingressBlock.Body()
					if ingress.FromPort != nil {
						ingressBody.SetAttributeValue(SG_FROM_PORT,
							cty.NumberIntVal(int64(*ingress.FromPort)))
					} else {
						ingressBody.SetAttributeValue(SG_FROM_PORT,
							cty.NumberIntVal(int64(0)))
					}
					if ingress.ToPort != nil {
						ingressBody.SetAttributeValue(SG_TO_PORT,
							cty.NumberIntVal(int64(*ingress.ToPort)))
					} else {
						ingressBody.SetAttributeValue(SG_TO_PORT,
							cty.NumberIntVal(int64(0)))
					}

					ingressBody.SetAttributeValue(SG_PROTOCOL,
						cty.StringVal(*ingress.IpProtocol))

					if len(ingress.IpRanges) > 0 {
						var vals []cty.Value
						for _, s := range ingress.IpRanges {
							vals = append(vals, cty.StringVal(*s.CidrIp))
						}
						ingressBody.SetAttributeValue(SG_CIDR_BLOCKS,
							cty.ListVal(vals))
					}
					if len(ingress.Ipv6Ranges) > 0 {
						var vals []cty.Value
						for _, s := range ingress.Ipv6Ranges {
							vals = append(vals, cty.StringVal(*s.CidrIpv6))
						}
						ingressBody.SetAttributeValue(SG_IPV6_CIDR_BLOCKS,
							cty.ListVal(vals))
					}
					if len(ingress.PrefixListIds) > 0 {
						var vals []cty.Value
						for _, s := range ingress.PrefixListIds {
							vals = append(vals, cty.StringVal(*s.PrefixListId))
						}
						ingressBody.SetAttributeValue(SG_PREFIX_LIST_IDS,
							cty.ListVal(vals))
					}
					if len(ingress.UserIdGroupPairs) > 0 {
						var vals []cty.Value
						sgid := ""
						desc := ""
						for _, s := range ingress.UserIdGroupPairs {
							vals = append(vals, cty.StringVal(*s.GroupId))
							sgid = *s.GroupId
							if s.Description != nil {
								desc = *s.Description
							}
						}

						if len(desc) > 0 {
							ingressBody.SetAttributeValue(SG_DESCRIPTION,
								cty.StringVal(desc))
						}
						if len(vals) == 1 && sgid == *sg.GroupId {
							ingressBody.SetAttributeValue(SG_SELF,
								cty.BoolVal(true))
						} else {

							ingressBody.SetAttributeValue(SG_SECURITY_GROUPS,
								cty.ListVal(vals))
						}
					}

				}
			}
			if len(sg.IpPermissionsEgress) > 0 {
				for _, egress := range sg.IpPermissionsEgress {
					egressBlock := sgBody.AppendNewBlock(SG_EGRESS,
						nil)
					egressBody := egressBlock.Body()
					if egress.FromPort != nil {
						egressBody.SetAttributeValue(SG_FROM_PORT,
							cty.NumberIntVal(int64(*egress.FromPort)))
					} else {
						egressBody.SetAttributeValue(SG_FROM_PORT,
							cty.NumberIntVal(int64(0)))
					}
					if egress.ToPort != nil {
						egressBody.SetAttributeValue(SG_TO_PORT,
							cty.NumberIntVal(int64(*egress.ToPort)))
					} else {
						egressBody.SetAttributeValue(SG_TO_PORT,
							cty.NumberIntVal(int64(0)))
					}

					egressBody.SetAttributeValue(SG_PROTOCOL,
						cty.StringVal(*egress.IpProtocol))

					if len(egress.IpRanges) > 0 {
						var vals []cty.Value
						for _, s := range egress.IpRanges {
							vals = append(vals, cty.StringVal(*s.CidrIp))
						}
						egressBody.SetAttributeValue(SG_CIDR_BLOCKS,
							cty.ListVal(vals))
					}
					if len(egress.Ipv6Ranges) > 0 {
						var vals []cty.Value
						for _, s := range egress.Ipv6Ranges {
							vals = append(vals, cty.StringVal(*s.CidrIpv6))
						}
						egressBody.SetAttributeValue(SG_IPV6_CIDR_BLOCKS,
							cty.ListVal(vals))
					}
					if len(egress.PrefixListIds) > 0 {
						var vals []cty.Value
						for _, s := range egress.PrefixListIds {
							vals = append(vals, cty.StringVal(*s.PrefixListId))
						}
						egressBody.SetAttributeValue(SG_PREFIX_LIST_IDS,
							cty.ListVal(vals))
					}
					if len(egress.UserIdGroupPairs) > 0 {
						var vals []cty.Value
						sgid := ""
						for _, s := range egress.UserIdGroupPairs {
							vals = append(vals, cty.StringVal(*s.GroupId))
							sgid = *s.GroupId
						}
						if len(vals) == 1 && sgid == *sg.GroupId {
							egressBody.SetAttributeValue(SG_SELF,
								cty.BoolVal(true))
						} else {
							egressBody.SetAttributeValue(SG_SECURITY_GROUPS,
								cty.ListVal(vals))
						}
					}

				}
			}

			if len(sg.Tags) > 0 {
				newMap := make(map[string]cty.Value)
				for _, tag := range sg.Tags {
					if common.IsTagAwsManaged(*tag.Key) {
						continue
					}
					newMap[*tag.Key] = cty.StringVal(*tag.Value)
				}
				sgBody.SetAttributeValue(TAGS, cty.MapVal(newMap))
			}
			if config.GenerateTfState {
				importConfigs = append(importConfigs, common.ImportConfig{
					ResourceAddress: strings.Join([]string{
						AWS_SECURITY_GROUP,
						resourceName,
					}, "."),
					ResourceId: *sg.GroupId,
					WorkingDir: workingDir,
				})
				tfContext.ImportConfigs = importConfigs
			}
			rootBody.AppendNewline()
			log.Printf("[TRACE] Terraform config generation done for aws security group (%s).", *sg.GroupName)
		}
		_, err = tfFile.Write(hclFile.Bytes())
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	}
	return &tfContext, nil
}
