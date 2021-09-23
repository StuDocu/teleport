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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"

	"github.com/gravitational/trace"
)

// Watcher watches cloud databases.
type Watcher interface {
	// Get returns cloud databases matching the watcher's selector.
	Get(context.Context) (types.Databases, error)
}

// RDSWatcherConfig is the cloud watcher configuration.
type RDSWatcherConfig struct {
	// Matchers is a list of selectors to match cloud databases.
	Matchers []services.RDSMatcher
	// RDS is the RDS API client.
	RDS rdsiface.RDSAPI
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *RDSWatcherConfig) CheckAndSetDefaults() error {
	if len(c.Matchers) == 0 {
		return trace.BadParameter("missing parameter Matchers")
	}
	if c.RDS == nil {
		return trace.BadParameter("missing parameter RDS")
	}
	return nil
}

// rdsWatcher watches cloud databases.
type rdsWatcher struct {
	cfg RDSWatcherConfig
}

// NewRDSWatcher returns a new cloud databases watcher instance.
func NewRDSWatcher(config RDSWatcherConfig) (Watcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &rdsWatcher{
		cfg: config,
	}, nil
}

// Get returns RDS and Aurora databases matching the watcher's selectors.
func (w *rdsWatcher) Get(ctx context.Context) (types.Databases, error) {
	out, err := w.cfg.RDS.DescribeDBInstancesWithContext(ctx, &rds.DescribeDBInstancesInput{
		Filters: []*rds.Filter{
			{
				Name:   aws.String("engine"),
				Values: aws.StringSlice([]string{"postgres", "mysql"}),
			},
		},
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
}
