package tfgenerator

import awsservices "tenant-native-terraform-generator/tf-generator/aws-services"

var TenantGenerators = []Generator{}

var AWSServicesGenerators = []Generator{
	&awsservices.EC2{},
}

var AppGenerators = []Generator{}
