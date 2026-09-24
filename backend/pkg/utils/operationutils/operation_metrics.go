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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"k8s.io/component-base/metrics/legacyregistry"

	"github.com/Azure/ARO-HCP/internal/api/coreapi"
	"github.com/Azure/ARO-HCP/internal/database/cosmosstorage/cosmosstorageutils"
)

var clusterProvisionCompletionsTotal *prometheus.CounterVec

func init() {
	clusterProvisionCompletionsTotal = newClusterProvisionCompletionsCounter(legacyregistry.Registerer())
}

func newClusterProvisionCompletionsCounter(r prometheus.Registerer) *prometheus.CounterVec {
	c := promauto.With(r).NewCounterVec(
		prometheus.CounterOpts{
			Name: "backend_cluster_provision_completions_total",
			Help: "Total cluster create completions by terminal phase.",
		},
		[]string{"phase"},
	)
	c.WithLabelValues("succeeded")
	c.WithLabelValues("failed")
	c.WithLabelValues("canceled")
	return c
}

func recordClusterProvisionCompletion(oldOp *coreapi.Operation, newStatus coreapi.ProvisioningState) {
	if oldOp == nil || oldOp.ExternalID == nil {
		return
	}
	if !strings.EqualFold(oldOp.ExternalID.ResourceType.String(), coreapi.ClusterResourceType.String()) {
		return
	}
	if oldOp.Request != cosmosstorageutils.OperationRequestCreate {
		return
	}
	if oldOp.Status.IsTerminal() {
		return
	}
	if !newStatus.IsTerminal() {
		return
	}

	clusterProvisionCompletionsTotal.WithLabelValues(strings.ToLower(string(newStatus))).Inc()
}
