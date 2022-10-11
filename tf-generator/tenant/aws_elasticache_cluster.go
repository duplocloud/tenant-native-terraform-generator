package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"tenant-native-terraform-generator/duplosdk"
	"tenant-native-terraform-generator/tf-generator/common"
)

const (
	REDIS                      string = "redis"
	CLUSTER_ID                 string = "cluster_id"
	ENGINE                     string = "engine"
	NODE_TYPE                  string = "node_type"
	NUM_CACHE_NODES            string = "num_cache_nodes"
	PARAMETER_GROUP_NAME       string = "parameter_group_name"
	ENGINE_VERSION             string = "engine_version"
	SUBNET_GROUP_NAME          string = "subnet_group_name"
	SNAPSHOT_ARNS              string = "snapshot_arns"
	SECURITY_GROUP_IDS         string = "security_group_ids"
	AZ_MODE                    string = "az_mode"
	REPLICATION_GROUP_ID       string = "replication_group_id"
	DESCRIPTION                string = "description"
	MULTI_AZ_ENABLED           string = "multi_az_enabled"
	NUM_CACHE_CLUSTERS         string = "num_cache_clusters"
	AUTOMATIC_FAILOVER_ENABLED string = "automatic_failover_enabled"
	AT_REST_ENCRYPTION_ENABLED string = "at_rest_encryption_enabled"
	TRANSIT_ENCRYPTION_ENABLED string = "transit_encryption_enabled"
)

const AWS_ELASTICACHE_REPLICATION_GROUP = "aws_elasticache_replication_group"
const AWS_ELASTICACHE_CLUSTER = "aws_elasticache_cluster"
const ELASTICACHE_PREFIX = "ecache_"
const ELASTICACHE_FILE_NAME_PREFIX = "aws-ecache-"

type AwsElasticacheCluster struct {
}

func (awsElasticacheCluster *AwsElasticacheCluster) Generate(config *common.Config, client *duplosdk.Client) (*common.TFContext, error) {
	workingDir := filepath.Join(config.TFCodePath, config.TenantProject)
	list, clientErr := client.EcacheInstanceList(config.TenantId)

	if clientErr != nil {
		fmt.Println(clientErr)
		return nil, nil
	}
	tfContext := common.TFContext{}
	importConfigs := []common.ImportConfig{}
	if list != nil && len(*list) > 0 {
		log.Println("[TRACE] <====== Ecache instance TF generation started. =====>")
		elasticacheClient := elasticache.NewFromConfig(config.AwsClientConfig)
		for _, cluster := range *list {
			if cluster.CacheType == 0 {
				replicationGroupsOutput, err := elasticacheClient.DescribeReplicationGroups(context.TODO(),
					&elasticache.DescribeReplicationGroupsInput{ReplicationGroupId: &cluster.Identifier})
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				b, err := json.Marshal(replicationGroupsOutput)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println("||==================================================================||")
				fmt.Println(string(b))
				fmt.Println("||==================================================================||")
				if replicationGroupsOutput != nil {
					for _, rg := range replicationGroupsOutput.ReplicationGroups {
						shortName := cluster.Identifier[len("duplo-"):len(cluster.Identifier)]
						resourceName := common.GetResourceName(shortName)

						hclFile := hclwrite.NewEmptyFile()

						path := filepath.Join(workingDir, ELASTICACHE_FILE_NAME_PREFIX+shortName+".tf")
						tfFile, err := os.Create(path)
						if err != nil {
							fmt.Println(err)
							return nil, err
						}
						rootBody := hclFile.Body()
						ecacheBlock := rootBody.AppendNewBlock("resource",
							[]string{AWS_ELASTICACHE_REPLICATION_GROUP,
								resourceName})
						ecacheBody := ecacheBlock.Body()
						ecacheBody.SetAttributeValue(REPLICATION_GROUP_ID,
							cty.StringVal(*rg.ReplicationGroupId))
						ecacheBody.SetAttributeValue(DESCRIPTION,
							cty.StringVal(*rg.Description))
						ecacheBody.SetAttributeValue(NODE_TYPE,
							cty.StringVal(*rg.CacheNodeType))
						ecacheBody.SetAttributeValue(NUM_CACHE_CLUSTERS,
							cty.NumberIntVal(int64(len(rg.MemberClusters))))
						ecacheBody.SetAttributeValue(ENGINE,
							cty.StringVal(REDIS))
						if string(rg.MultiAZ) == "enabled" {
							ecacheBody.SetAttributeValue(MULTI_AZ_ENABLED,
								cty.BoolVal(true))
						}
						if string(rg.AutomaticFailover) == "enabled" {
							ecacheBody.SetAttributeValue(AUTOMATIC_FAILOVER_ENABLED,
								cty.BoolVal(true))
						}
						if rg.AtRestEncryptionEnabled != nil && *rg.AtRestEncryptionEnabled {
							ecacheBody.SetAttributeValue(AT_REST_ENCRYPTION_ENABLED,
								cty.BoolVal(true))
						}
						if rg.TransitEncryptionEnabled != nil && *rg.TransitEncryptionEnabled {
							ecacheBody.SetAttributeValue(TRANSIT_ENCRYPTION_ENABLED,
								cty.BoolVal(true))
						}
						if rg.KmsKeyId != nil {
							ecacheBody.SetAttributeValue(KMS_KEY_ID,
								cty.StringVal(*rg.KmsKeyId))
						}

						if len(rg.MemberClusters) > 0 {
							cacheClusters, err := elasticacheClient.DescribeCacheClusters(context.TODO(), &elasticache.DescribeCacheClustersInput{
								CacheClusterId: &rg.MemberClusters[0],
							})
							if err != nil {
								fmt.Println(err)
								return nil, err
							}
							if cacheClusters != nil && len(cacheClusters.CacheClusters) > 0 {
								cluster := cacheClusters.CacheClusters[0]
								if cluster.EngineVersion != nil {
									parts := strings.Split(*cluster.EngineVersion, ".")
									if len(parts) == 3 {
										ecacheBody.SetAttributeValue(ENGINE_VERSION,
											cty.StringVal(strings.Join([]string{parts[0], parts[1]}, ".")))
									} else {
										ecacheBody.SetAttributeValue(ENGINE_VERSION,
											cty.StringVal(*cluster.EngineVersion))
									}
								}
								if cluster.CacheParameterGroup != nil {
									ecacheBody.SetAttributeValue(PARAMETER_GROUP_NAME,
										cty.StringVal(*cluster.CacheParameterGroup.CacheParameterGroupName))
								}
								if len(cluster.SecurityGroups) > 0 {
									var vals []cty.Value
									for _, s := range cluster.SecurityGroups {
										vals = append(vals, cty.StringVal(*s.SecurityGroupId))
									}
									ecacheBody.SetAttributeValue(SECURITY_GROUP_IDS,
										cty.ListVal(vals))
								}
								if cluster.CacheSubnetGroupName != nil {
									ecacheBody.SetAttributeValue(SUBNET_GROUP_NAME,
										cty.StringVal(*cluster.CacheSubnetGroupName))
								}
								tagsOutput, err := elasticacheClient.ListTagsForResource(context.TODO(), &elasticache.ListTagsForResourceInput{
									ResourceName: cluster.ARN,
								})
								if err != nil {
									fmt.Println(err)
									return nil, err
								}
								if tagsOutput != nil {
									if len(tagsOutput.TagList) > 0 {
										tagsTokens := hclwrite.Tokens{
											{Type: hclsyntax.TokenOQuote, Bytes: []byte(`{`)},
											{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
										}
										for _, tag := range tagsOutput.TagList {
											tagValue := strings.Replace(*tag.Value, config.TenantName, "${local.tenant_name}", -1)
											tag := "\"" + *tag.Key + "\"" + " = \"" + tagValue + "\"\n"
											tagsTokens = append(tagsTokens,
												&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(tag)},
											)
										}
										tagsTokens = append(tagsTokens, &hclwrite.Token{Type: hclsyntax.TokenCQuote, Bytes: []byte(`}`)})
										ecacheBody.SetAttributeRaw(TAGS, tagsTokens)
									}
								}
							}
						}

						_, err = tfFile.Write(hclFile.Bytes())
						if err != nil {
							fmt.Println(err)
							return nil, err
						}
						if config.GenerateTfState {
							importConfigs = append(importConfigs, common.ImportConfig{
								ResourceAddress: strings.Join([]string{
									AWS_ELASTICACHE_REPLICATION_GROUP,
									resourceName,
								}, "."),
								ResourceId: *rg.ReplicationGroupId,
								WorkingDir: workingDir,
							})
							tfContext.ImportConfigs = importConfigs
						}
					}
				}
			} else {
				cacheClusters, err := elasticacheClient.DescribeCacheClusters(context.TODO(),
					&elasticache.DescribeCacheClustersInput{CacheClusterId: &cluster.Identifier})
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				if cacheClusters != nil && len(cacheClusters.CacheClusters) > 0 {
					b, err := json.Marshal(cacheClusters)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Println("||==================================================================||")
					fmt.Println(string(b))
					fmt.Println("||==================================================================||")
				}
			}
		}
		log.Println("[TRACE] <====== Ecache instance TF generation done. =====>")
	}
	return &tfContext, nil
}
