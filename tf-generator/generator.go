package tfgenerator

import (
	"tenant-native-terraform-generator/duplosdk"

	"tenant-native-terraform-generator/tf-generator/common"
)

type Generator interface {
	Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error)
}
