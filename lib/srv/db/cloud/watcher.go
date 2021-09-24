/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Watcher watches cloud databases.
type Watcher interface {
	// Get returns cloud databases matching the watcher's selector.
	Get(context.Context) (types.Databases, error)
}

// RDSWatcherConfig is the cloud watcher configuration.
type RDSWatcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// RDS is the RDS API client.
	RDS rdsiface.RDSAPI
	// Region is the AWS region to query databases in.
	Region string
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *RDSWatcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.RDS == nil {
		return trace.BadParameter("missing parameter RDS")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// rdsWatcher watches cloud databases.
type rdsWatcher struct {
	cfg RDSWatcherConfig
	log logrus.FieldLogger
}

// NewRDSWatcher returns a new cloud databases watcher instance.
func NewRDSWatcher(config RDSWatcherConfig) (Watcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &rdsWatcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "rds-watcher",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns RDS and Aurora databases matching the watcher's selectors.
func (w *rdsWatcher) Get(ctx context.Context) (types.Databases, error) {
	rdsDatabases, err := w.getRDSDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auroraDatabases, err := w.getAuroraDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result types.Databases
	for _, database := range append(rdsDatabases, auroraDatabases...) {
		match, _, err := services.MatchLabels(w.cfg.Labels, database.GetAllLabels())
		if err != nil {
			w.log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			result = append(result, database)
		} else {
			w.log.Debugf("%v doesn't match selector.", database)
		}
	}
	return result, nil
}

func (w *rdsWatcher) getRDSDatabases(ctx context.Context) (types.Databases, error) {
	// TODO(r0mant): Support pagination.
	out, err := w.cfg.RDS.DescribeDBInstancesWithContext(ctx, &rds.DescribeDBInstancesInput{
		Filters: rdsFilters(),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	databases := make(types.Databases, 0, len(out.DBInstances))
	for _, instance := range out.DBInstances {
		database, err := newDatabaseFromRDSInstance(instance)
		if err != nil {
			w.log.Infof("Could not convert RDS instance %q to database resource: %v.",
				aws.StringValue(instance.DBInstanceIdentifier), err)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// newDatabaseFromRDSInstance makes a database resource from RDS instance.
func newDatabaseFromRDSInstance(instance *rds.DBInstance) (types.Database, error) {
	endpoint := instance.Endpoint
	if endpoint == nil {
		return nil, trace.BadParameter("empty endpoint")
	}
	metadata, err := metadataFromRDSInstance(instance)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(types.Metadata{
		Name:        aws.StringValue(instance.DBInstanceIdentifier),
		Description: fmt.Sprintf("RDS instance %v in %v", metadata.RDS.InstanceID, metadata.Region),
		Labels:      tagsToLabels(instance.TagList),
	}, types.DatabaseSpecV3{
		Protocol: engineToProtocol(aws.StringValue(instance.Engine)),
		URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint.Address), aws.Int64Value(endpoint.Port)),
		AWS:      *metadata,
	})
}

func (w *rdsWatcher) getAuroraDatabases(ctx context.Context) (types.Databases, error) {
	// TODO(r0mant): Support pagination.
	out, err := w.cfg.RDS.DescribeDBClustersWithContext(ctx, &rds.DescribeDBClustersInput{
		Filters: auroraFilters(),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	databases := make(types.Databases, 0, len(out.DBClusters))
	for _, cluster := range out.DBClusters {
		database, err := newDatabaseFromRDSCluster(cluster)
		if err != nil {
			w.log.Infof("Could not convert RDS cluster %q to database resource: %v.",
				aws.StringValue(cluster.DBClusterIdentifier), err)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// newDatabaseFromRDSCluster makes a database resource from RDS cluster (Aurora).
func newDatabaseFromRDSCluster(cluster *rds.DBCluster) (types.Database, error) {
	metadata, err := metadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(types.Metadata{
		Name:        aws.StringValue(cluster.DBClusterIdentifier),
		Description: fmt.Sprintf("Aurora cluster %v in %v", metadata.RDS.ClusterID, metadata.Region),
		Labels:      tagsToLabels(cluster.TagList),
	}, types.DatabaseSpecV3{
		Protocol: engineToProtocol(aws.StringValue(cluster.Engine)),
		URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.Endpoint), aws.Int64Value(cluster.Port)),
		AWS:      *metadata,
	})
}

func rdsFilters() []*rds.Filter {
	return []*rds.Filter{{
		Name: aws.String("engine"),
		Values: aws.StringSlice([]string{
			enginePostgres, engineMySQL}),
	}}
}

func auroraFilters() []*rds.Filter {
	return []*rds.Filter{{
		Name: aws.String("engine"),
		Values: aws.StringSlice([]string{
			engineAurora, engineAuroraMySQL, engineAuroraPostgres}),
	}}
}

func engineToProtocol(engine string) string {
	switch engine {
	case enginePostgres, engineAuroraPostgres:
		return defaults.ProtocolPostgres
	case engineMySQL, engineAurora, engineAuroraMySQL:
		return defaults.ProtocolMySQL
	}
	return ""
}

func tagsToLabels(tags []*rds.Tag) map[string]string {
	labels := make(map[string]string)
	for _, tag := range tags {
		labels[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return labels
}

const (
	// engineMySQL is RDS engine name for MySQL instances.
	engineMySQL = "mysql"
	// enginePostgres is RDS engine name for Postgres instances.
	enginePostgres = "postgres"
	// engineAurora is RDS engine name for Aurora MySQL 5.6 compatible clusters.
	engineAurora = "aurora"
	// engineAuroraMySQL is RDS engine name for Aurora MySQL 5.7 compatible clusters.
	engineAuroraMySQL = "aurora-mysql"
	// engineAuroraPostgres is RDS engine name for Aurora Postgres clusters.
	engineAuroraPostgres = "aurora-postgresql"
)
