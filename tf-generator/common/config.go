package common

import "github.com/aws/aws-sdk-go-v2/aws"

type Config struct {
	DuploHost          string
	DuploToken         string
	TenantId           string
	TenantName         string
	TenantPlanName     string
	CustomerName       string
	AdminTenantDir     string
	AwsProviderVersion string
	TenantProject      string
	GenerateTfState    bool
	S3Backend          bool
	S3Bucket           string
	DynamodbTable      string
	ValidateTf         bool
	AccountID          string
	TFCodePath         string
	TFVersion          string
	AwsRegion          string
	AwsClientConfig    aws.Config
}

type TFContext struct {
	TargetLocation string
	InputVars      []VarConfig
	OutputVars     []OutputVarConfig
	ImportConfigs  []ImportConfig
}
