package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/hashicorp/hcl/v2"
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
)

const AWS_INSTANCE = "aws_instance"
const EC2_VAR_PREFIX = "ec2_instance_"
const EC2_FILE_NAME_PREFIX = "aws-instance-"

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
										Name: "aws_key_pair.tenant_keypair",
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
							newMap := make(map[string]cty.Value)
							for _, tag := range instance.Tags {
								newMap[*tag.Key] = cty.StringVal(*tag.Value)
							}
							ec2Body.SetAttributeValue(TAGS, cty.MapVal(newMap))
						}

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
