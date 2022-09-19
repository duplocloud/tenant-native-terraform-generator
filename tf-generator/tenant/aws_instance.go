package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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

const EC2_VAR_PREFIX = "ec2_"
const FILE_NAME_PREFIX = "ec2-"

type EC2 struct {
}

func (ec2Instance *EC2) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	list, clientErr := client.NativeHostGetList(config.TenantId)
	//Get tenant from duplo

	if clientErr != nil {
		fmt.Println(clientErr)
		return nil, nil
	}
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	if list != nil {
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
		svc := ec2.NewFromConfig(config.AwsClientConfig)
		resp, err := svc.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{InstanceIds: instanceIds})
		if err != nil {
			fmt.Println(clientErr)
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

						path := filepath.Join(workingDir, FILE_NAME_PREFIX+shortName+".tf")
						tfFile, err := os.Create(path)
						if err != nil {
							fmt.Println(err)
							return nil, err
						}
						// initialize the body of the new file object
						rootBody := hclFile.Body()

						// Add aws_instance resource
						ec2Block := rootBody.AppendNewBlock("resource",
							[]string{"aws_instance",
								resourceName})
						ec2Body := ec2Block.Body()
						ec2Body.SetAttributeTraversal("ami", hcl.Traversal{
							hcl.TraverseRoot{
								Name: "var",
							},
							hcl.TraverseAttr{
								Name: varFullPrefix + "ami",
							},
						})
						ec2Body.SetAttributeTraversal("instance_type", hcl.Traversal{
							hcl.TraverseRoot{
								Name: "var",
							},
							hcl.TraverseAttr{
								Name: varFullPrefix + "instance_type",
							},
						})
						ec2Body.SetAttributeValue("availability_zone",
							cty.StringVal(*instance.Placement.AvailabilityZone))
						// ec2Body.SetAttributeValue("iam_instance_profile",
						// 	cty.StringVal(*instance.IamInstanceProfile.Id))

						if len(instance.Tags) > 0 {
							newMap := make(map[string]cty.Value)
							for _, tag := range instance.Tags {
								newMap[*tag.Key] = cty.StringVal(*tag.Value)
							}
							ec2Body.SetAttributeValue("tags", cty.MapVal(newMap))
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
								ResourceAddress: "aws_instance." + resourceName,
								ResourceId:      *instance.InstanceId,
								WorkingDir:      workingDir,
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
		Name:          prefix + "private_ip",
		ActualVal:     "aws_instance." + resourceName + ".private_ip",
		DescVal:       "The AWS EC2 instance Private IP.",
		RootTraversal: true,
	}
	outVarConfigs["private_ip"] = var1
	var2 := common.OutputVarConfig{
		Name:          prefix + "aws_instance",
		ActualVal:     "aws_instance." + resourceName + ".public_ip",
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
