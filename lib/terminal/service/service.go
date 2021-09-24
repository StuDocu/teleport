// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package terminalv1

import (
	"context"
	"fmt"

	v1 "github.com/gravitational/teleport/lib/terminal/api/protogen/golang/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Service implements teleport.terminal.v1.TerminalService.
type Service struct {
}

// RemoveInvite removes a single invite token
func (s *Service) ListClusters(ctx context.Context, r *v1.ListClustersRequest) (*v1.ListClustersResponse, error) {

	fmt.Print("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	result := v1.ListClustersResponse{
		NextPageToken: "fsdfsdf",
	}

	return &result, nil
}

// POST /clusters
func (s *Service) CreateCluster(context.Context, *v1.CreateClusterRequest) (*v1.Cluster, error) {
	return nil, nil
}

// TODO(codingllama): Names may change!
// POST /clusters/{cluster_id}/loginChallenges
func (s *Service) CreateClusterLoginChallenge(context.Context, *v1.CreateClusterLoginChallengeRequest) (*v1.ClusterLoginChallenge, error) {
	return nil, nil
}

// POST /clusters/{cluster_id}/loginChallenges/{challenge_id}:solve
func (s *Service) SolveClusterLoginChallenge(context.Context, *v1.SolveClusterLoginChallengeRequest) (*v1.SolveClusterLoginChallengeResponse, error) {
	return nil, nil
}

// GET /databases
// Requires login challenge to be solved beforehand.
func (s *Service) ListDatabases(context.Context, *v1.ListDatabasesRequest) (*v1.ListDatabasesResponse, error) {
	return nil, nil
}

// POST /gateways
func (s *Service) CreateGateway(context.Context, *v1.CreateGatewayRequest) (*v1.Gateway, error) {
	return nil, nil
}

// GET /gateways
func (s *Service) ListGateways(context.Context, *v1.ListGatewaysRequest) (*v1.ListGatewaysResponse, error) {
	return nil, nil
}

// DELETE /gateways/{id}
func (s *Service) DeleteGateway(context.Context, *v1.DeleteGatewayRequest) (*emptypb.Empty, error) {
	return nil, nil
}

// Streams input/output using a gateway.
// Requires the gateway to be created beforehand.
// This has no REST counterpart.
func (s *Service) StreamGateway(v1.TerminalService_StreamGatewayServer) error {
	return nil
}

// GET /nodes
// Per Teleport nomenclature, a Node is an SSH-capable node.
// Requires login challenge to be solved beforehand.
func (s *Service) ListNodes(context.Context, *v1.ListNodesRequest) (*v1.ListNodesResponse, error) {
	return nil, nil
}
