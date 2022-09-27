package common

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

type IValidator interface {
	Validate() (*Config, error)
}

type EnvVarValidator struct {
}

func (envVar *EnvVarValidator) Validate() (*Config, error) {
	host := os.Getenv("duplo_host")
	if len(host) == 0 {
		err := fmt.Errorf("error - Please provide \"%s\" as env variable", "duplo_host")
		log.Printf("[TRACE] - %s", err)
		return nil, err
	}
	token := os.Getenv("duplo_token")
	if len(token) == 0 {
		err := fmt.Errorf("error - Please provide \"%s\" as env variable", "duplo_token")
		log.Printf("[TRACE] - %s", err)
		return nil, err
	}

	tenantName := os.Getenv("tenant_name")
	if len(tenantName) == 0 {
		err := fmt.Errorf("error - please provide \"%s\" as env variable", "tenant_name")
		log.Printf("[TRACE] - %s", err)
		return nil, err
	}
	custName := os.Getenv("customer_name")
	if len(custName) == 0 {
		err := fmt.Errorf("error - please provide \"%s\" as env variable", "customer_name")
		log.Printf("[TRACE] - %s", err)
		return nil, err
	}

	awsProviderVersion := os.Getenv("aws_provider_version")
	if len(awsProviderVersion) == 0 {
		awsProviderVersion = "4.30.0"
	}

	tfVersion := os.Getenv("tf_version")
	if len(tfVersion) == 0 {
		tfVersion = "0.14.11"
	}

	tenantProject := os.Getenv("tenant_project")
	if len(tenantProject) == 0 {
		tenantProject = "tenant"
	}

	generateTfState := false

	generateTfStateStr := os.Getenv("generate_tf_state")
	if len(generateTfStateStr) == 0 {
		generateTfState = false
	} else {
		generateTfStateBool, err := strconv.ParseBool(generateTfStateStr)
		if err != nil {
			err = fmt.Errorf("error while reading generate_tf_state from env vars %s", err)
			log.Printf("[TRACE] - %s", err)
			return nil, err
		}
		generateTfState = generateTfStateBool
	}

	validateTf := true
	validateTfStr := os.Getenv("validate_tf")
	if len(validateTfStr) == 0 {
		validateTf = true
	} else {
		validateTf, _ = strconv.ParseBool(generateTfStateStr)
	}

	s3Backend := false
	s3Bucket := ""
	dynamodbTable := ""
	s3BackendStr := os.Getenv("s3_backend")
	if len(s3BackendStr) == 0 {
		s3Backend = false
	} else {
		s3BackendBool, err := strconv.ParseBool(s3BackendStr)
		if err != nil {
			err = fmt.Errorf("error while reading s3_backend from env vars %s", err)
			log.Printf("[TRACE] - %s", err)
			return nil, err
		}
		s3Backend = s3BackendBool
		if s3Backend {
			s3Bucket = os.Getenv("s3_bucket")
			if len(s3Bucket) == 0 {
				err := fmt.Errorf("error - Please provide \"%s\" as env variable", "s3_bucket")
				log.Printf("[TRACE] - %s", err)
				return nil, err
			}
			dynamodbTable = os.Getenv("dynamodb_table")
		}
	}

	return &Config{
		DuploHost:          host,
		DuploToken:         token,
		TenantName:         tenantName,
		CustomerName:       custName,
		AwsProviderVersion: awsProviderVersion,
		TenantProject:      tenantProject,
		GenerateTfState:    generateTfState,
		S3Backend:          s3Backend,
		S3Bucket:           s3Bucket,
		DynamodbTable:      dynamodbTable,
		ValidateTf:         validateTf,
		TFVersion:          tfVersion,
	}, nil
}
