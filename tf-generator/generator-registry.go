package tfgenerator

import (
	"tenant-native-terraform-generator/tf-generator/tenant"
)

var TenantGenerators = []Generator{
	&tenant.AwsVars{},
	&tenant.TenantMain{},
	&tenant.TenantKeyPair{},
	&tenant.TenantKMS{},
	&tenant.TenantIAM{},
	&tenant.TenantSG{},
	&tenant.AwsInstance{},
}
