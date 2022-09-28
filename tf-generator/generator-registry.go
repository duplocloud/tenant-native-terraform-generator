package tfgenerator

import (
	"tenant-native-terraform-generator/tf-generator/tenant"
)

var TenantGenerators = []Generator{
	&tenant.AwsVars{},
	&tenant.TenantIAM{},
	&tenant.AwsInstance{},
}
