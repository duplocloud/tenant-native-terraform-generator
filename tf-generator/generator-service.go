package tfgenerator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"tenant-native-terraform-generator/duplosdk"
	"tenant-native-terraform-generator/tf-generator/common"
	"tenant-native-terraform-generator/tf-generator/tenant"
)

type IGeneratorService interface {
	PreProcess(config *common.Config, client *duplosdk.Client) error
	StartTFGeneration(config *common.Config, client *duplosdk.Client) error
	PostProcess(config *common.Config, client *duplosdk.Client) error
}

type TfGeneratorService struct {
}

func (tfg *TfGeneratorService) PreProcess(config *common.Config, client *duplosdk.Client) error {
	log.Println("[TRACE] <====== Initialize target directory with customer name and tenant id. =====>")
	config.TFCodePath = filepath.Join("target", config.CustomerName, config.TenantName)
	tenantProject := filepath.Join(config.TFCodePath, config.TenantProject)
	err := os.RemoveAll(filepath.Join("target", config.CustomerName, config.TenantName))
	if err != nil {
		log.Fatal(err)
	}
	err = os.RemoveAll(tenantProject)
	if err != nil {
		log.Fatal(err)
	}
	err = os.MkdirAll(tenantProject, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	config.AdminTenantDir = tenantProject

	err = duplosdk.Copy(".gitignore", filepath.Join("target", config.CustomerName, config.TenantName, ".gitignore"))
	if err != nil {
		log.Fatal(err)
	}
	err = duplosdk.Copy(".envrc", filepath.Join("target", config.CustomerName, config.TenantName, ".envrc"))
	if err != nil {
		log.Fatal(err)
	}
	envFile, err := os.OpenFile(filepath.Join("target", config.CustomerName, config.TenantName, ".envrc"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer envFile.Close()
	if _, err := envFile.WriteString("\nexport tenant_id=\"" + config.TenantId + "\""); err != nil {
		log.Fatal(err)
	}
	log.Println("[TRACE] <====== Initialized target directory with customer name and tenant id. =====>")
	return nil
}

func (tfg *TfGeneratorService) StartTFGeneration(config *common.Config, client *duplosdk.Client) error {
	// var tf *tfexec.Terraform
	providerGen := &common.Provider{}
	providerGen.Generate(config, client)

	// if config.GenerateTfState {
	// 	tf := tfInit(config, config.AdminTenantDir)
	// 	tfNewWorkspace(config, tf)
	// }

	log.Println("[TRACE] <====== Start TF generation for tenant project. =====>")
	// Register New TF generator for Tenant Project
	tenantGeneratorList := TenantGenerators
	if config.S3Backend {
		tenantGeneratorList = append(tenantGeneratorList, &tenant.TenantBackend{})
	}

	starTFGenerationForProject(config, client, tenantGeneratorList, config.AdminTenantDir)
	if config.ValidateTf {
		common.ValidateAndFormatTfCode(config.AdminTenantDir, config.TFVersion)
	}
	log.Println("[TRACE] <====== End TF generation for tenant project. =====>")

	return nil
}

func starTFGenerationForProject(config *common.Config, client *duplosdk.Client, generatorList []Generator, targetLocation string) {

	tfContext := common.TFContext{
		TargetLocation: targetLocation,
		InputVars:      []common.VarConfig{},
		OutputVars:     []common.OutputVarConfig{},
	}

	// 1. Generate Duplo TF resources.
	for _, g := range generatorList {
		c, err := g.Generate(config, client)
		if err != nil {
			log.Fatalf("error running admin tenant tf generation: %s", err)
		}
		if c != nil {
			if len(c.InputVars) > 0 {
				tfContext.InputVars = append(tfContext.InputVars, c.InputVars...)
			}
			if len(c.OutputVars) > 0 {
				tfContext.OutputVars = append(tfContext.OutputVars, c.OutputVars...)
			}
			if len(c.ImportConfigs) > 0 {
				tfContext.ImportConfigs = append(tfContext.ImportConfigs, c.ImportConfigs...)
			}
		}
	}
	// 2. Generate input vars.
	if len(tfContext.InputVars) > 0 {
		varsGenerator := common.Vars{
			TargetLocation: tfContext.TargetLocation,
			Vars:           tfContext.InputVars,
		}
		varsGenerator.Generate()
	}
	// 3. Generate output vars.
	if len(tfContext.OutputVars) > 0 {
		outVarsGenerator := common.OutputVars{
			TargetLocation: tfContext.TargetLocation,
			OutputVars:     tfContext.OutputVars,
		}
		outVarsGenerator.Generate()
	}
	// 4. Import all resources
	if config.GenerateTfState && len(tfContext.ImportConfigs) > 0 {
		tfInitializer := common.TfInitializer{
			WorkingDir: targetLocation,
			Config:     config,
		}
		tf := tfInitializer.InitWithWorkspace()
		importer := &common.Importer{}
		// Get state file if already present.
		state, err := tf.Show(context.Background())
		if err != nil {
			// log.Fatalf("error running Show: %s", err)
			fmt.Println(err)
		}
		importedResourceAddresses := []string{}
		if state != nil && state.Values != nil && state.Values.RootModule != nil && len(state.Values.RootModule.Resources) > 0 {
			for _, r := range state.Values.RootModule.Resources {
				importedResourceAddresses = append(importedResourceAddresses, r.Address)
			}
		}
		for _, ic := range tfContext.ImportConfigs {
			//importer.Import(config, &ic)
			if common.Contains(importedResourceAddresses, ic.ResourceAddress) {
				log.Printf("[TRACE] Resource %s is already imported.", ic.ResourceAddress)
				continue
			}
			importer.ImportWithoutInit(config, &ic, tf)
		}
		//tfInitializer.DeleteWorkspace(config, tf)
	}
}

func (tfg *TfGeneratorService) PostProcess(config *common.Config, client *duplosdk.Client) error {
	return nil
}
