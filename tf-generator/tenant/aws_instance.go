package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"os"
	"path/filepath"
	"tenant-native-terraform-generator/duplosdk"
	"tenant-native-terraform-generator/tf-generator/common"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	AMI                         string = "ami"
	INSTANCE_TYPE               string = "instance_type"
	AVAILABILITY_ZONE           string = "availability_zone"
	TAGS                        string = "tags"
	ASSOCIATE_PUBLIC_IP_ADDRESS string = "associate_public_ip_address"
	IAM_INSTANCE_PROFILE        string = "iam_instance_profile"
	VPC_SECURITY_GROUP_IDS      string = "vpc_security_group_ids"
	SUBNET_ID                   string = "subnet_id"
	KEY_NAME                    string = "key_name"
	EBS_OPTIMIZED               string = "ebs_optimized"
	USER_DATA_BASE64            string = "user_data_base64"
	ENCRYPTED                   string = "encrypted"
	IOPS                        string = "iops"
	SNAPSHOT_ID                 string = "snapshot_id"
	TYPE                        string = "type"
	SIZE                        string = "size"
	KMS_KEY_ID                  string = "kms_key_id"
	THROUGHPUT                  string = "throughput"
	DEVICE_NAME                 string = "device_name"
	VOLUME_ID                   string = "volume_id"
	INSTANCE_ID                 string = "instance_id"
	HIBERNATION                 string = "hibernation"
)

const AWS_INSTANCE = "aws_instance"
const EC2_VAR_PREFIX = "ec2_instance_"
const EC2_FILE_NAME_PREFIX = "aws-instance-"
const AWS_EBS_VOLUME = "aws_ebs_volume"
const AWS_VOLUME_ATTACHMENT = "aws_volume_attachment"

type AwsInstance struct {
}

func (ec2Instance *AwsInstance) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	list, clientErr := client.NativeHostGetList(config.TenantId)
	//Get tenant from duplo

	if clientErr != nil {
		fmt.Println(clientErr)
		return nil, nil
	}
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	if list != nil && len(*list) > 0 {
		instanceIdNameMap := map[string]string{}
		instanceIds := []string{}

		for _, host := range *list {
			if isPartOfAsg(host) {
				continue
			}
			shortName := host.FriendlyName[len("duploservices-"+config.TenantName+"-"):len(host.FriendlyName)]
			instanceIdNameMap[host.InstanceID] = shortName
			instanceIds = append(instanceIds, host.InstanceID)
		}
		if len(instanceIds) > 0 {
			ec2Client := ec2.NewFromConfig(config.AwsClientConfig)
			resp, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{InstanceIds: instanceIds})
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			b, err := json.Marshal(resp)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("||==================================================================||")
			fmt.Println(string(b))
			fmt.Println("||==================================================================||")

			log.Println("[TRACE] <====== EC2 instance TF generation started. =====>")
			if resp != nil && len(resp.Reservations) > 0 {
				for _, reservation := range resp.Reservations {
					if len(reservation.Instances) > 0 {
						for _, instance := range reservation.Instances {
							shortName := instanceIdNameMap[*instance.InstanceId]
							resourceName := common.GetResourceName(shortName)

							varFullPrefix := EC2_VAR_PREFIX + resourceName + "_"
							inputVars := generateEC2InstanceVars(instance, varFullPrefix)
							tfContext.InputVars = append(tfContext.InputVars, inputVars...)
							// create new empty hcl file object
							hclFile := hclwrite.NewEmptyFile()

							path := filepath.Join(workingDir, EC2_FILE_NAME_PREFIX+shortName+".tf")
							tfFile, err := os.Create(path)
							if err != nil {
								fmt.Println(err)
								return nil, err
							}
							// initialize the body of the new file object
							rootBody := hclFile.Body()

							// Add aws_instance resource
							ec2Block := rootBody.AppendNewBlock("resource",
								[]string{AWS_INSTANCE,
									resourceName})
							ec2Body := ec2Block.Body()
							ec2Body.SetAttributeTraversal(AMI, hcl.Traversal{
								hcl.TraverseRoot{
									Name: "var",
								},
								hcl.TraverseAttr{
									Name: varFullPrefix + "ami",
								},
							})
							ec2Body.SetAttributeTraversal(INSTANCE_TYPE, hcl.Traversal{
								hcl.TraverseRoot{
									Name: "var",
								},
								hcl.TraverseAttr{
									Name: varFullPrefix + "instance_type",
								},
							})
							ec2Body.SetAttributeValue(AVAILABILITY_ZONE,
								cty.StringVal(*instance.Placement.AvailabilityZone))
							if instance.IamInstanceProfile != nil && instance.IamInstanceProfile.Arn != nil {
								roleName := strings.SplitN(*instance.IamInstanceProfile.Arn, ":instance-profile/", 2)[1]
								if "duploservices-"+config.TenantName == roleName {
									ec2Body.SetAttributeTraversal(IAM_INSTANCE_PROFILE, hcl.Traversal{
										hcl.TraverseRoot{
											Name: AWS_IAM_ROLE + "." + TENANT_IAM,
										},
										hcl.TraverseAttr{
											Name: "name",
										},
									})
								} else {
									ec2Body.SetAttributeValue(IAM_INSTANCE_PROFILE,
										cty.StringVal(roleName))
								}
							}

							ec2Body.SetAttributeValue(AVAILABILITY_ZONE,
								cty.StringVal(*instance.Placement.AvailabilityZone))

							if instance.HibernationOptions != nil && instance.HibernationOptions.Configured != nil {
								ec2Body.SetAttributeValue(HIBERNATION,
									cty.BoolVal(*instance.HibernationOptions.Configured))
							}
							if len(instance.SecurityGroups) > 0 {
								var vals []cty.Value
								for _, s := range instance.SecurityGroups {
									vals = append(vals, cty.StringVal(*s.GroupId))
								}
								ec2Body.SetAttributeValue(VPC_SECURITY_GROUP_IDS,
									cty.ListVal(vals))
							}

							if instance.SubnetId != nil {
								ec2Body.SetAttributeValue(SUBNET_ID,
									cty.StringVal(*instance.SubnetId))
							}
							if instance.KeyName != nil {
								if "duploservices-"+config.TenantName == *instance.KeyName {
									ec2Body.SetAttributeTraversal(KEY_NAME, hcl.Traversal{
										hcl.TraverseRoot{
											Name: AWS_KEY_PAIR + ".tenant_keypair",
										},
										hcl.TraverseAttr{
											Name: "key_name",
										},
									})
								} else {
									ec2Body.SetAttributeValue(KEY_NAME,
										cty.StringVal(*instance.KeyName))
								}
							}
							if instance.EbsOptimized != nil && *instance.EbsOptimized {
								ec2Body.SetAttributeValue(EBS_OPTIMIZED,
									cty.BoolVal(*instance.EbsOptimized))
							}

							if len(instance.Tags) > 0 {
								tagsTokens := hclwrite.Tokens{
									{Type: hclsyntax.TokenOQuote, Bytes: []byte(`{`)},
									{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
								}
								for _, tag := range instance.Tags {
									if common.IsTagAwsManaged(*tag.Key) {
										continue
									}
									tagValue := strings.Replace(*tag.Value, config.TenantName, "${local.tenant_name}", -1)
									tag := "\"" + *tag.Key + "\"" + " = \"" + tagValue + "\"\n"
									tagsTokens = append(tagsTokens,
										&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(tag)},
									)
								}
								tagsTokens = append(tagsTokens, &hclwrite.Token{Type: hclsyntax.TokenCQuote, Bytes: []byte(`}`)})
								ec2Body.SetAttributeRaw(TAGS, tagsTokens)
							}

							instanceAttributeOutput, err := ec2Client.DescribeInstanceAttribute(context.TODO(), &ec2.DescribeInstanceAttributeInput{
								Attribute: types.InstanceAttributeNameUserData, InstanceId: instance.InstanceId,
							})
							if err != nil {
								fmt.Println(err)
							}
							if instanceAttributeOutput != nil && instanceAttributeOutput.UserData != nil && instanceAttributeOutput.UserData.Value != nil {
								// data, err := base64.StdEncoding.DecodeString(*instanceAttributeOutput.UserData.Value)
								// if err != nil {
								// 	log.Fatal("error:", err)
								// }
								ec2Body.SetAttributeValue(USER_DATA_BASE64,
									cty.StringVal(*instanceAttributeOutput.UserData.Value))
							}

							// Add aws_ebs_volume
							if len(instance.BlockDeviceMappings) > 0 {
								volIds := []string{}
								volIdDevice := map[string]string{}
								for _, ebs := range instance.BlockDeviceMappings {
									volIds = append(volIds, *ebs.Ebs.VolumeId)
									volIdDevice[*ebs.Ebs.VolumeId] = *ebs.DeviceName
								}
								volumesOutput, err := ec2Client.DescribeVolumes(context.TODO(), &ec2.DescribeVolumesInput{VolumeIds: volIds})
								if err != nil {
									fmt.Println(err)
									return nil, err
								}
								if volumesOutput != nil && len(volumesOutput.Volumes) > 0 {
									for _, vol := range volumesOutput.Volumes {
										rootBody.AppendNewline()
										ebsVolBlock := rootBody.AppendNewBlock("resource",
											[]string{AWS_EBS_VOLUME,
												resourceName + "_ebs_vol"})
										ebsVolBody := ebsVolBlock.Body()
										ebsVolBody.SetAttributeValue(AVAILABILITY_ZONE,
											cty.StringVal(*vol.AvailabilityZone))
										if vol.Encrypted != nil {
											ebsVolBody.SetAttributeValue(ENCRYPTED,
												cty.BoolVal(*vol.Encrypted))
										}
										if vol.Iops != nil {
											ebsVolBody.SetAttributeValue(IOPS,
												cty.NumberIntVal(int64(*vol.Iops)))
										}
										if vol.SnapshotId != nil {
											ebsVolBody.SetAttributeValue(SNAPSHOT_ID,
												cty.StringVal(*vol.SnapshotId))
										}
										if vol.Size != nil {
											ebsVolBody.SetAttributeValue(SIZE,
												cty.NumberIntVal(int64(*vol.Size)))
										}
										if len(vol.VolumeType) > 0 {
											ebsVolBody.SetAttributeValue(TYPE,
												cty.StringVal(string(vol.VolumeType)))
										}
										if vol.KmsKeyId != nil {
											ebsVolBody.SetAttributeValue(KMS_KEY_ID,
												cty.StringVal(*vol.KmsKeyId))
										}
										if vol.Throughput != nil {
											ebsVolBody.SetAttributeValue(THROUGHPUT,
												cty.NumberIntVal(int64(*vol.Throughput)))
										}
										if len(vol.Tags) > 0 {
											newMap := make(map[string]cty.Value)
											for _, tag := range vol.Tags {
												//tagValue := strings.Replace(*tag.Value, config.TenantName, "${local.tenant_name}", -1)
												newMap[*tag.Key] = cty.StringVal(*tag.Value)
											}
											ebsVolBody.SetAttributeValue(TAGS, cty.MapVal(newMap))
										}

										if config.GenerateTfState {
											importConfigs = append(importConfigs, common.ImportConfig{
												ResourceAddress: strings.Join([]string{
													AWS_EBS_VOLUME,
													resourceName + "_ebs_vol",
												}, "."),
												ResourceId: *vol.VolumeId,
												WorkingDir: workingDir,
											})
											tfContext.ImportConfigs = importConfigs
										}
										rootBody.AppendNewline()
										ebsVolAttachBlock := rootBody.AppendNewBlock("resource",
											[]string{AWS_VOLUME_ATTACHMENT,
												resourceName + "_ebs_vol_attach"})
										ebsVolAttachBody := ebsVolAttachBlock.Body()
										ebsVolAttachBody.SetAttributeValue(DEVICE_NAME,
											cty.StringVal(volIdDevice[*vol.VolumeId]))

										ebsVolAttachBody.SetAttributeTraversal(VOLUME_ID, hcl.Traversal{
											hcl.TraverseRoot{
												Name: AWS_EBS_VOLUME + "." + resourceName + "_ebs_vol",
											},
											hcl.TraverseAttr{
												Name: "id",
											},
										})
										ebsVolAttachBody.SetAttributeTraversal(INSTANCE_ID, hcl.Traversal{
											hcl.TraverseRoot{
												Name: AWS_INSTANCE + "." + resourceName,
											},
											hcl.TraverseAttr{
												Name: "id",
											},
										})

										if config.GenerateTfState {
											importConfigs = append(importConfigs, common.ImportConfig{
												ResourceAddress: strings.Join([]string{
													AWS_VOLUME_ATTACHMENT,
													resourceName + "_ebs_vol_attach",
												}, "."),
												ResourceId: strings.Join([]string{
													volIdDevice[*vol.VolumeId],
													*vol.VolumeId,
													*instance.InstanceId,
												}, ":"),
												WorkingDir: workingDir,
											})
											tfContext.ImportConfigs = importConfigs
										}
										break
									}
								}
							}
							lifecycleBlock := ec2Body.AppendNewBlock("lifecycle", nil)
							lifecycleBody := lifecycleBlock.Body()
							ignoreChanges := "user_data,user_data_base64,user_data_replace_on_change"
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
							log.Printf("[TRACE] Terraform config is generated for ec2 instance : %s", shortName)

							outVars := generateEC2InstanceOutputVars(varFullPrefix, resourceName)
							tfContext.OutputVars = append(tfContext.OutputVars, outVars...)

							// Import all created resources.
							if config.GenerateTfState {
								importConfigs = append(importConfigs, common.ImportConfig{
									ResourceAddress: strings.Join([]string{
										AWS_INSTANCE,
										resourceName,
									}, "."),
									ResourceId: *instance.InstanceId,
									WorkingDir: workingDir,
								})
								tfContext.ImportConfigs = importConfigs
							}
						}
					}
				}
			}
			log.Println("[TRACE] <====== EC2 instance TF generation done. =====>")
		}
	}
	return &tfContext, nil
}

func isPartOfAsg(host duplosdk.DuploNativeHost) bool {
	asgTagKey := []string{"aws:autoscaling:groupName"}
	if host.Tags != nil && len(*host.Tags) > 0 {
		asgTag := duplosdk.SelectKeyValues(host.Tags, asgTagKey)
		if asgTag != nil && len(*asgTag) > 0 {
			return true
		}
	}
	return false
}

func generateEC2InstanceVars(instance types.Instance, prefix string) []common.VarConfig {
	varConfigs := make(map[string]common.VarConfig)

	imageIdVar := common.VarConfig{
		Name:       prefix + "ami",
		DefaultVal: *instance.ImageId,
		TypeVal:    "string",
	}
	varConfigs["ami"] = imageIdVar

	capacityVar := common.VarConfig{
		Name:       prefix + "instance_type",
		DefaultVal: string(instance.InstanceType),
		TypeVal:    "string",
	}
	varConfigs["instance_type"] = capacityVar

	vars := make([]common.VarConfig, len(varConfigs))
	for _, v := range varConfigs {
		vars = append(vars, v)
	}
	return vars
}

func generateEC2InstanceOutputVars(prefix, resourceName string) []common.OutputVarConfig {
	outVarConfigs := make(map[string]common.OutputVarConfig)

	var1 := common.OutputVarConfig{
		Name: prefix + "private_ip",
		ActualVal: strings.Join([]string{
			AWS_INSTANCE,
			resourceName,
			"private_ip",
		}, "."),
		DescVal:       "The AWS EC2 instance Private IP.",
		RootTraversal: true,
	}
	outVarConfigs["private_ip"] = var1
	var2 := common.OutputVarConfig{
		Name: prefix + "public_ip",
		ActualVal: strings.Join([]string{
			AWS_INSTANCE,
			resourceName,
			"public_ip",
		}, "."),
		DescVal:       "The public IP address assigned to the instance.",
		RootTraversal: true,
	}
	outVarConfigs["public_ip"] = var2

	outVars := make([]common.OutputVarConfig, len(outVarConfigs))
	for _, v := range outVarConfigs {
		outVars = append(outVars, v)
	}
	return outVars
}
