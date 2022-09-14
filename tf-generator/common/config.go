package common

import "github.com/aws/aws-sdk-go-v2/aws"

type Config struct {
	DuploHost          string
	DuploToken         string
	TenantId           string
	TenantName         string
	CertArn            string
	CustomerName       string
	AdminTenantDir     string
	AwsServicesDir     string
	AppDir             string
	AwsProviderVersion string
	TenantProject      string
	AwsServicesProject string
	AppProject         string
	GenerateTfState    bool
	S3Backend          bool
	ValidateTf         bool
	AccountID          string
	TFCodePath         string
	TFVersion          string
	SkipAdminTenant    bool
	SkipAwsServices    bool
	SkipApp            bool
	AwsRegion          string
	AwsClientConfig    aws.Config
}

type TFContext struct {
	TargetLocation string
	InputVars      []VarConfig
	OutputVars     []OutputVarConfig
	ImportConfigs  []ImportConfig
}
