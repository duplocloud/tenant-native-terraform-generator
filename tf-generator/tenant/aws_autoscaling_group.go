package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"tenant-native-terraform-generator/duplosdk"
	"tenant-native-terraform-generator/tf-generator/common"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

const (
	ASG_NAME                    string = "name"
	IMAGE_ID                    string = "image_id"
	TAG                         string = "tag"
	KEY                         string = "key"
	VALUE                       string = "value"
	PROPAGATE_AT_LAUNCH         string = "propagate_at_launch"
	AVAILABILITY_ZONES          string = "availability_zones"
	DESIRED_CAPACITY            string = "desired_capacity"
	MAX_SIZE                    string = "max_size"
	MIN_SIZE                    string = "min_size"
	HEALTH_CHECK_TYPE           string = "health_check_type"
	CAPACITY_REBALANCE          string = "capacity_rebalance"
	VPC_ZONE_IDENTIFIER         string = "vpc_zone_identifier"
	TERMINATION_POLICIES        string = "termination_policies"
	SECURITY_GROUPS             string = "security_groups"
	EBS_BLOCK_DEVICE            string = "ebs_block_device"
	DELETE_ON_TERMINATION       string = "delete_on_termination"
	LAUNCH_CONFIGURATION        string = "launch_configuration"
	METADATA_OPTIONS            string = "metadata_options"
	HTTP_ENDPOINT               string = "http_endpoint"
	HTTP_PUT_RESPONSE_HOP_LIMIT string = "http_put_response_hop_limit"
	HTTP_TOKENS                 string = "http_tokens"
	HEALTH_CHECK_GRACE_PERIOD   string = "health_check_grace_period"
	VOLUME_SIZE                 string = "volume_size"
	VOLUME_TYPE                 string = "volume_type"
)

const AWS_AUTOSCALING_GROUP = "aws_autoscaling_group"
const AWS_LAUNCH_CONFIGURATION = "aws_launch_configuration"
const ASG_PREFIX = "asg_"
const ASG_FILE_NAME_PREFIX = "aws-asg-"

type AwsASG struct {
}

func (awsASG *AwsASG) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	list, clientErr := client.AsgProfileGetList(config.TenantId)

	if clientErr != nil {
		fmt.Println(clientErr)
		return nil, nil
	}
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	if list != nil && len(*list) > 0 {
		log.Println("[TRACE] <====== Autoscaling group TF generation started. =====>")
		asgGroupNames := []string{}
		for _, asg := range *list {
			asgGroupNames = append(asgGroupNames, asg.FriendlyName)
		}
		asgClient := autoscaling.NewFromConfig(config.AwsClientConfig)
		input := &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: asgGroupNames,
		}

		autoScalingGroupsOutput, err := asgClient.DescribeAutoScalingGroups(context.TODO(), input)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		if autoScalingGroupsOutput != nil && len(autoScalingGroupsOutput.AutoScalingGroups) > 0 {
			for _, asgGroup := range autoScalingGroupsOutput.AutoScalingGroups {

				friendlyName := *asgGroup.AutoScalingGroupName
				shortName := friendlyName[len("duploservices-"+config.TenantName+"-"):len(*asgGroup.AutoScalingGroupName)]
				resourceName := common.GetResourceName(shortName)

				varFullPrefix := ASG_PREFIX + resourceName + "_"
				inputVars := generateASGVars(asgGroup, varFullPrefix)
				tfContext.InputVars = append(tfContext.InputVars, inputVars...)
				hclFile := hclwrite.NewEmptyFile()

				path := filepath.Join(workingDir, ASG_FILE_NAME_PREFIX+shortName+".tf")
				tfFile, err := os.Create(path)
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				rootBody := hclFile.Body()

				asgBlock := rootBody.AppendNewBlock("resource",
					[]string{AWS_AUTOSCALING_GROUP,
						resourceName})
				asgBody := asgBlock.Body()
				asgBody.SetAttributeTraversal(ASG_NAME, hcl.Traversal{
					hcl.TraverseRoot{
						Name: "var",
					},
					hcl.TraverseAttr{
						Name: varFullPrefix + "name",
					},
				})

				asgBody.SetAttributeValue(MAX_SIZE,
					cty.NumberIntVal(int64(*asgGroup.MaxSize)))
				asgBody.SetAttributeValue(MIN_SIZE,
					cty.NumberIntVal(int64(*asgGroup.MinSize)))
				if asgGroup.DesiredCapacity != nil && *asgGroup.DesiredCapacity > 0 {
					asgBody.SetAttributeValue(DESIRED_CAPACITY,
						cty.NumberIntVal(int64(*asgGroup.DesiredCapacity)))
				}
				if asgGroup.HealthCheckGracePeriod != nil {
					asgBody.SetAttributeValue(HEALTH_CHECK_GRACE_PERIOD,
						cty.NumberIntVal(int64(*asgGroup.HealthCheckGracePeriod)))
				} else {
					asgBody.SetAttributeValue(HEALTH_CHECK_GRACE_PERIOD,
						cty.NumberIntVal(int64(300)))
				}
				if asgGroup.VPCZoneIdentifier != nil {
					vals := []cty.Value{cty.StringVal(*asgGroup.VPCZoneIdentifier)}
					asgBody.SetAttributeValue(VPC_ZONE_IDENTIFIER,
						cty.ListVal(vals))
				} else {
					if len(asgGroup.AvailabilityZones) > 0 {
						var vals []cty.Value
						for _, s := range asgGroup.AvailabilityZones {
							vals = append(vals, cty.StringVal(s))
						}
						asgBody.SetAttributeValue(AVAILABILITY_ZONES,
							cty.ListVal(vals))
					}
				}
				if asgGroup.HealthCheckType != nil {
					asgBody.SetAttributeValue(HEALTH_CHECK_TYPE,
						cty.StringVal(*asgGroup.HealthCheckType))
				}
				// if len(asgGroup.TerminationPolicies) > 0 {
				// 	var vals []cty.Value
				// 	for _, s := range asgGroup.TerminationPolicies {
				// 		vals = append(vals, cty.StringVal(s))
				// 	}
				// 	asgBody.SetAttributeValue(TERMINATION_POLICIES,
				// 		cty.ListVal(vals))
				// }
				if len(asgGroup.Tags) > 0 {
					for _, tag := range asgGroup.Tags {
						//tagValue := strings.Replace(*tag.Value, config.TenantName, "${local.tenant_name}", -1)
						if common.IsTagAwsManaged(*tag.Key) {
							continue
						}
						tagBlock := asgBody.AppendNewBlock(TAG,
							nil)
						tagBody := tagBlock.Body()
						tagBody.SetAttributeValue(KEY,
							cty.StringVal(*tag.Key))
						if config.TenantName == *tag.Value {
							tagBody.SetAttributeTraversal(VALUE, hcl.Traversal{
								hcl.TraverseRoot{
									Name: "local",
								},
								hcl.TraverseAttr{
									Name: "tenant_name",
								},
							})
						} else {
							tagValue := strings.Replace(*tag.Value, config.TenantName, "${local.tenant_name}", -1)
							tagTokens := hclwrite.Tokens{
								{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)},
								{Type: hclsyntax.TokenIdent, Bytes: []byte(tagValue)},
								{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)},
							}
							tagBody.SetAttributeRaw(VALUE, tagTokens)
						}

						tagBody.SetAttributeValue(PROPAGATE_AT_LAUNCH,
							cty.BoolVal(*tag.PropagateAtLaunch))
					}
				}

				if asgGroup.LaunchConfigurationName != nil {
					asgBody.SetAttributeTraversal(LAUNCH_CONFIGURATION, hcl.Traversal{
						hcl.TraverseRoot{
							Name: AWS_LAUNCH_CONFIGURATION + "." + resourceName + "_lc",
						},
						hcl.TraverseAttr{
							Name: "name",
						},
					})

					launchConfigurationsOutput, err := asgClient.DescribeLaunchConfigurations(context.TODO(), &autoscaling.DescribeLaunchConfigurationsInput{
						LaunchConfigurationNames: []string{*asgGroup.LaunchConfigurationName},
					})
					if err != nil {
						fmt.Println(err)
						return nil, err
					}
					b, err := json.Marshal(launchConfigurationsOutput)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Println("||==================================================================||")
					fmt.Println(string(b))
					fmt.Println("||==================================================================||")
					for _, lc := range launchConfigurationsOutput.LaunchConfigurations {
						rootBody.AppendNewline()
						lcBlock := rootBody.AppendNewBlock("resource",
							[]string{AWS_LAUNCH_CONFIGURATION,
								resourceName + "_lc"})
						lcBody := lcBlock.Body()

						lcBody.SetAttributeTraversal(ASG_NAME, hcl.Traversal{
							hcl.TraverseRoot{
								Name: "var",
							},
							hcl.TraverseAttr{
								Name: varFullPrefix + "name",
							},
						})
						lcBody.SetAttributeValue(IMAGE_ID,
							cty.StringVal(*lc.ImageId))
						lcBody.SetAttributeValue(INSTANCE_TYPE,
							cty.StringVal(*lc.InstanceType))
						if lc.AssociatePublicIpAddress != nil {
							lcBody.SetAttributeValue(ASSOCIATE_PUBLIC_IP_ADDRESS,
								cty.BoolVal(*lc.AssociatePublicIpAddress))
						}
						if lc.IamInstanceProfile != nil {
							if "duploservices-"+config.TenantName == *lc.IamInstanceProfile {
								lcBody.SetAttributeTraversal(IAM_INSTANCE_PROFILE, hcl.Traversal{
									hcl.TraverseRoot{
										Name: AWS_IAM_ROLE + "." + TENANT_IAM,
									},
									hcl.TraverseAttr{
										Name: "name",
									},
								})
							} else {
								lcBody.SetAttributeValue(IAM_INSTANCE_PROFILE,
									cty.StringVal(*lc.IamInstanceProfile))
							}
						}
						if lc.KeyName != nil {
							if "duploservices-"+config.TenantName == *lc.KeyName {
								lcBody.SetAttributeTraversal(KEY_NAME, hcl.Traversal{
									hcl.TraverseRoot{
										Name: AWS_KEY_PAIR + ".tenant_keypair",
									},
									hcl.TraverseAttr{
										Name: "key_name",
									},
								})
							} else {
								lcBody.SetAttributeValue(KEY_NAME,
									cty.StringVal(*lc.KeyName))
							}
						}
						if lc.EbsOptimized != nil && *lc.EbsOptimized {
							lcBody.SetAttributeValue(EBS_OPTIMIZED,
								cty.BoolVal(*lc.EbsOptimized))
						}
						if lc.UserData != nil {
							lcBody.SetAttributeValue(USER_DATA_BASE64,
								cty.StringVal(*lc.UserData))
						}
						if len(lc.SecurityGroups) > 0 {
							var vals []cty.Value
							for _, s := range lc.SecurityGroups {
								vals = append(vals, cty.StringVal(s))
							}
							lcBody.SetAttributeValue(SECURITY_GROUPS,
								cty.ListVal(vals))
						}
						if lc.MetadataOptions != nil {
							mdoBlock := lcBody.AppendNewBlock(METADATA_OPTIONS,
								nil)
							mdoBody := mdoBlock.Body()
							mdo := lc.MetadataOptions
							if len(mdo.HttpEndpoint) > 0 {
								mdoBody.SetAttributeValue(HTTP_ENDPOINT,
									cty.StringVal(string(mdo.HttpEndpoint)))
							}
							if mdo.HttpPutResponseHopLimit != nil {
								mdoBody.SetAttributeValue(HTTP_PUT_RESPONSE_HOP_LIMIT,
									cty.NumberIntVal(int64(*mdo.HttpPutResponseHopLimit)))
							}
							if len(mdo.HttpTokens) > 0 {
								mdoBody.SetAttributeValue(HTTP_TOKENS,
									cty.StringVal(string(mdo.HttpTokens)))
							}
						}
						if len(lc.BlockDeviceMappings) > 0 {
							for _, bdm := range lc.BlockDeviceMappings {
								bdmBlock := lcBody.AppendNewBlock(EBS_BLOCK_DEVICE,
									nil)
								bdmBody := bdmBlock.Body()

								bdmBody.SetAttributeValue(DEVICE_NAME,
									cty.StringVal(*bdm.DeviceName))
								if bdm.Ebs != nil {
									ebs := bdm.Ebs
									if ebs.Encrypted != nil {
										bdmBody.SetAttributeValue(ENCRYPTED,
											cty.BoolVal(*ebs.Encrypted))
									}
									if ebs.Iops != nil {
										bdmBody.SetAttributeValue(IOPS,
											cty.NumberIntVal(int64(*ebs.Iops)))
									}
									if ebs.SnapshotId != nil {
										bdmBody.SetAttributeValue(SNAPSHOT_ID,
											cty.StringVal(*ebs.SnapshotId))
									}
									if ebs.VolumeSize != nil {
										bdmBody.SetAttributeValue(VOLUME_SIZE,
											cty.NumberIntVal(int64(*ebs.VolumeSize)))
									}
									if ebs.VolumeType != nil {
										bdmBody.SetAttributeValue(VOLUME_TYPE,
											cty.StringVal(*ebs.VolumeType))
									}
									if ebs.Throughput != nil {
										bdmBody.SetAttributeValue(THROUGHPUT,
											cty.NumberIntVal(int64(*ebs.Throughput)))
									}
									if ebs.DeleteOnTermination != nil {
										bdmBody.SetAttributeValue(DELETE_ON_TERMINATION,
											cty.BoolVal(*ebs.DeleteOnTermination))
									} else {
										bdmBody.SetAttributeValue(DELETE_ON_TERMINATION,
											cty.BoolVal(false))
									}
								}
							}

						}
						lifecycleBlock := lcBody.AppendNewBlock("lifecycle", nil)
						lifecycleBody := lifecycleBlock.Body()
						ignoreChanges := "user_data, user_data_base64"
						ignoreChangesTokens := hclwrite.Tokens{
							{Type: hclsyntax.TokenOQuote, Bytes: []byte(`[`)},
							{Type: hclsyntax.TokenIdent, Bytes: []byte(ignoreChanges)},
							{Type: hclsyntax.TokenCQuote, Bytes: []byte(`]`)},
						}
						lifecycleBody.SetAttributeRaw("ignore_changes", ignoreChangesTokens)
						if config.GenerateTfState {
							importConfigs = append(importConfigs, common.ImportConfig{
								ResourceAddress: strings.Join([]string{
									AWS_LAUNCH_CONFIGURATION,
									resourceName + "_lc",
								}, "."),
								ResourceId: *lc.LaunchConfigurationName,
								WorkingDir: workingDir,
							})
							tfContext.ImportConfigs = importConfigs
						}
					}
				}

				lifecycleBlock := asgBody.AppendNewBlock("lifecycle", nil)
				lifecycleBody := lifecycleBlock.Body()
				ignoreChanges := "force_delete,force_delete_warm_pool,wait_for_capacity_timeout"
				ignoreChangesTokens := hclwrite.Tokens{
					{Type: hclsyntax.TokenOQuote, Bytes: []byte(`[`)},
					{Type: hclsyntax.TokenIdent, Bytes: []byte(ignoreChanges)},
					{Type: hclsyntax.TokenCQuote, Bytes: []byte(`]`)},
				}
				lifecycleBody.SetAttributeRaw("ignore_changes", ignoreChangesTokens)

				_, err = tfFile.Write(hclFile.Bytes())
				if err != nil {
					fmt.Println(err)
					return nil, err
				}

				if config.GenerateTfState {
					importConfigs = append(importConfigs, common.ImportConfig{
						ResourceAddress: strings.Join([]string{
							AWS_AUTOSCALING_GROUP,
							resourceName,
						}, "."),
						ResourceId: *asgGroup.AutoScalingGroupName,
						WorkingDir: workingDir,
					})
					tfContext.ImportConfigs = importConfigs
				}
			}
		}
		log.Println("[TRACE] <====== Autoscaling group TF generation done. =====>")
	}
	return &tfContext, nil
}

func generateASGVars(asg types.AutoScalingGroup, prefix string) []common.VarConfig {
	varConfigs := make(map[string]common.VarConfig)

	imageIdVar := common.VarConfig{
		Name:       prefix + "name",
		DefaultVal: *asg.AutoScalingGroupName,
		TypeVal:    "string",
	}
	varConfigs["name"] = imageIdVar

	vars := make([]common.VarConfig, len(varConfigs))
	for _, v := range varConfigs {
		vars = append(vars, v)
	}
	return vars
}
