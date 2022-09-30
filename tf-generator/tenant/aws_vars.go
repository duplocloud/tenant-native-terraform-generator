package tenant

import (
	"tenant-native-terraform-generator/duplosdk"
	"tenant-native-terraform-generator/tf-generator/common"
)

type AwsVars struct {
}

func (awsVars *AwsVars) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	tfContext := common.TFContext{}
	varConfigs := make(map[string]common.VarConfig)
	infraConfig, _ := client.InfrastructureGetConfig(config.TenantPlanName)

	vpcVar := common.VarConfig{
		Name:       "vpc_id",
		DefaultVal: infraConfig.Vnet.ID,
		TypeVal:    "string",
	}
	varConfigs["vpc_id"] = vpcVar

	tenantVar := common.VarConfig{
		Name:       "tenant_name",
		DefaultVal: config.TenantName,
		TypeVal:    "string",
	}
	varConfigs["tenant_name"] = tenantVar

	regionVar := common.VarConfig{
		Name:       "region",
		DefaultVal: config.AwsRegion,
		TypeVal:    "string",
	}
	varConfigs["region"] = regionVar

	vars := make([]common.VarConfig, len(varConfigs))
	for _, v := range varConfigs {
		vars = append(vars, v)
	}

	tfContext.InputVars = vars
	return &tfContext, nil
}
