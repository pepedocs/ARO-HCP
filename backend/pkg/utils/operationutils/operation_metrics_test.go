// Copyright 2026 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operationutils

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	azcorearm "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"

	"github.com/Azure/ARO-HCP/internal/api/coreapi"
	"github.com/Azure/ARO-HCP/internal/api/metadataapi"
	"github.com/Azure/ARO-HCP/internal/database/cosmosstorage/cosmosstorageutils"
)

func clusterCreateOp(status coreapi.ProvisioningState) *coreapi.Operation {
	return &coreapi.Operation{
		ExternalID: metadataapi.Must(azcorearm.ParseResourceID(
			"/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/cluster-1",
		)),
		Request: cosmosstorageutils.OperationRequestCreate,
		Status:  status,
	}
}

func nodepoolCreateOp(status coreapi.ProvisioningState) *coreapi.Operation {
	return &coreapi.Operation{
		ExternalID: metadataapi.Must(azcorearm.ParseResourceID(
			"/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/cluster-1/nodePools/np-1",
		)),
		Request: cosmosstorageutils.OperationRequestCreate,
		Status:  status,
	}
}

func setupTestCounter(t *testing.T) *prometheus.CounterVec {
	t.Helper()
	reg := prometheus.NewRegistry()
	counter := newClusterProvisionCompletionsCounter(reg)
	original := clusterProvisionCompletionsTotal
	clusterProvisionCompletionsTotal = counter
	t.Cleanup(func() { clusterProvisionCompletionsTotal = original })
	return counter
}

func TestRecordClusterProvisionCompletion_Increments(t *testing.T) {
	tests := []struct {
		name     string
		oldState coreapi.ProvisioningState
		newState coreapi.ProvisioningState
		phase    string
	}{
		{
			name:     "provisioning to succeeded",
			oldState: coreapi.ProvisioningStateProvisioning,
			newState: coreapi.ProvisioningStateSucceeded,
			phase:    "succeeded",
		},
		{
			name:     "provisioning to failed",
			oldState: coreapi.ProvisioningStateProvisioning,
			newState: coreapi.ProvisioningStateFailed,
			phase:    "failed",
		},
		{
			name:     "provisioning to canceled",
			oldState: coreapi.ProvisioningStateProvisioning,
			newState: coreapi.ProvisioningStateCanceled,
			phase:    "canceled",
		},
		{
			name:     "accepted to succeeded",
			oldState: coreapi.ProvisioningStateAccepted,
			newState: coreapi.ProvisioningStateSucceeded,
			phase:    "succeeded",
		},
		{
			name:     "accepted to failed",
			oldState: coreapi.ProvisioningStateAccepted,
			newState: coreapi.ProvisioningStateFailed,
			phase:    "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := setupTestCounter(t)

			op := clusterCreateOp(tt.oldState)
			recordClusterProvisionCompletion(op, tt.newState)

			require.Equal(t, float64(1), testutil.ToFloat64(counter.WithLabelValues(tt.phase)))
		})
	}
}

func TestRecordClusterProvisionCompletion_DoesNotIncrement(t *testing.T) {
	tests := []struct {
		name     string
		op       *coreapi.Operation
		newState coreapi.ProvisioningState
	}{
		{
			name:     "non-terminal to non-terminal",
			op:       clusterCreateOp(coreapi.ProvisioningStateAccepted),
			newState: coreapi.ProvisioningStateProvisioning,
		},
		{
			name:     "terminal to same terminal",
			op:       clusterCreateOp(coreapi.ProvisioningStateFailed),
			newState: coreapi.ProvisioningStateFailed,
		},
		{
			name:     "terminal to different terminal",
			op:       clusterCreateOp(coreapi.ProvisioningStateFailed),
			newState: coreapi.ProvisioningStateSucceeded,
		},
		{
			name: "cluster update",
			op: &coreapi.Operation{
				ExternalID: metadataapi.Must(azcorearm.ParseResourceID(
					"/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/cluster-1",
				)),
				Request: cosmosstorageutils.OperationRequestUpdate,
				Status:  coreapi.ProvisioningStateProvisioning,
			},
			newState: coreapi.ProvisioningStateSucceeded,
		},
		{
			name: "cluster delete",
			op: &coreapi.Operation{
				ExternalID: metadataapi.Must(azcorearm.ParseResourceID(
					"/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/cluster-1",
				)),
				Request: cosmosstorageutils.OperationRequestDelete,
				Status:  coreapi.ProvisioningStateDeleting,
			},
			newState: coreapi.ProvisioningStateSucceeded,
		},
		{
			name:     "nodepool create",
			op:       nodepoolCreateOp(coreapi.ProvisioningStateProvisioning),
			newState: coreapi.ProvisioningStateSucceeded,
		},
		{
			name:     "nil operation",
			op:       nil,
			newState: coreapi.ProvisioningStateSucceeded,
		},
		{
			name: "nil external ID",
			op: &coreapi.Operation{
				ExternalID: nil,
				Request:    cosmosstorageutils.OperationRequestCreate,
				Status:     coreapi.ProvisioningStateProvisioning,
			},
			newState: coreapi.ProvisioningStateSucceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := setupTestCounter(t)

			recordClusterProvisionCompletion(tt.op, tt.newState)

			for _, phase := range []string{"succeeded", "failed", "canceled"} {
				require.Equal(t, float64(0), testutil.ToFloat64(counter.WithLabelValues(phase)),
					"phase %q should not have been incremented", phase)
			}
		})
	}
}

func TestClusterProvisionCompletionsInitializedAtZero(t *testing.T) {
	counter := setupTestCounter(t)

	expected := `
		# HELP backend_cluster_provision_completions_total Total cluster create completions by terminal phase.
		# TYPE backend_cluster_provision_completions_total counter
		backend_cluster_provision_completions_total{phase="canceled"} 0
		backend_cluster_provision_completions_total{phase="failed"} 0
		backend_cluster_provision_completions_total{phase="succeeded"} 0
	`
	require.NoError(t, testutil.CollectAndCompare(counter, strings.NewReader(expected), "backend_cluster_provision_completions_total"))
}
