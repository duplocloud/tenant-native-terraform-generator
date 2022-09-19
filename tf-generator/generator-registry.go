package tfgenerator

import (
	"tenant-native-terraform-generator/tf-generator/tenant"
)

var TenantGenerators = []Generator{}

var AWSServicesGenerators = []Generator{
	&tenant.EC2{},
}

var AppGenerators = []Generator{}
