package tfgenerator

import (
	"tenant-native-terraform-generator/tf-generator/tenant"
)

var TenantGenerators = []Generator{
	&tenant.AwsVars{},
	&tenant.TenantMain{},
	&tenant.TenantIAM{},
	&tenant.TenantSG{},
	&tenant.AwsInstance{},
}
