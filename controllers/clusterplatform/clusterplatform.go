/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package clusterplatform

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apache/incubator-kie-kogito-serverless-operator/api/metadata"
	operatorapi "github.com/apache/incubator-kie-kogito-serverless-operator/api/v1alpha08"
	"github.com/apache/incubator-kie-kogito-serverless-operator/log"
	"github.com/apache/incubator-kie-kogito-serverless-operator/utils"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultPlatformName is the standard name used for the platform.
	DefaultPlatformName = "kogito-serverless-platform"

	OperatorWatchNamespaceEnvVariable = "WATCH_NAMESPACE"
	operatorNamespaceEnvVariable      = "NAMESPACE"
)

// Copied from https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry

// LocalRegistryHostingV1 describes a local registry that developer tools can
// connect to. A local registry allows clients to load images into the local
// cluster by pushing to this registry.
type LocalRegistryHostingV1 struct {
	// Host documents the host (hostname and port) of the registry, as seen from
	// outside the cluster.
	//
	// This is the registry host that tools outside the cluster should push images
	// to.
	Host string `yaml:"host,omitempty"`

	// HostFromClusterNetwork documents the host (hostname and port) of the
	// registry, as seen from networking inside the container pods.
	//
	// This is the registry host that tools running on pods inside the cluster
	// should push images to. If not set, then tools inside the cluster should
	// assume the local registry is not available to them.
	HostFromClusterNetwork string `yaml:"hostFromClusterNetwork,omitempty"`

	// HostFromContainerRuntime documents the host (hostname and port) of the
	// registry, as seen from the cluster's container runtime.
	//
	// When tools apply Kubernetes objects to the cluster, this host should be
	// used for image name fields. If not set, users of this field should use the
	// value of Host instead.
	//
	// Note that it doesn't make sense semantically to define this field, but not
	// define Host or HostFromClusterNetwork. That would imply a way to pull
	// images without a way to push images.
	HostFromContainerRuntime string `yaml:"hostFromContainerRuntime,omitempty"`

	// Help contains a URL pointing to documentation for users on how to set
	// up and configure a local registry.
	//
	// Tools can use this to nudge users to enable the registry. When possible,
	// the writer should use as permanent a URL as possible to prevent drift
	// (e.g., a version control SHA).
	//
	// When image pushes to a registry host specified in one of the other fields
	// fail, the tool should display this help URL to the user. The help URL
	// should contain instructions on how to diagnose broken or misconfigured
	// registries.
	Help string `yaml:"help,omitempty"`
}

const OperatorLockName = "kogito-serverless-lock"

// IsCurrentOperatorGlobal returns true if the operator is configured to watch all namespaces.
func IsCurrentOperatorGlobal() bool {
	if watchNamespace, envSet := os.LookupEnv(OperatorWatchNamespaceEnvVariable); !envSet || strings.TrimSpace(watchNamespace) == "" {
		return true
	}
	return false
}

// GetOperatorNamespace returns the namespace where the current operator is located (if set).
func GetOperatorNamespace() string {
	if podNamespace, envSet := os.LookupEnv(operatorNamespaceEnvVariable); envSet {
		return podNamespace
	}
	return ""
}

// GetOperatorLockName returns the name of the lock lease that is electing a leader on the particular namepsace.
func GetOperatorLockName(operatorID string) string {
	return fmt.Sprintf("%s-lock", operatorID)
}

// GetActiveClusterPlatform returns the currently installed active cluster platform.
func GetActiveClusterPlatform(ctx context.Context, c ctrl.Client) (*operatorapi.SonataFlowClusterPlatform, error) {
	return getClusterPlatform(ctx, c, true)
}

// getClusterPlatform returns the currently installed cluster platform or any platform existing in the cluster.
func getClusterPlatform(ctx context.Context, c ctrl.Client, active bool) (*operatorapi.SonataFlowClusterPlatform, error) {
	klog.V(log.D).InfoS("Finding available cluster platforms")

	lst, err := listPrimaryClusterPlatforms(ctx, c)
	if err != nil {
		return nil, err
	}

	for _, p := range lst.Items {
		cPlatform := p // pin
		if IsActive(&cPlatform) {
			klog.V(log.D).InfoS("Found active cluster platform", "platform", cPlatform.Name)
			return &cPlatform, nil
		}
	}

	if !active && len(lst.Items) > 0 {
		// does not require the platform to be active, just return one if present
		res := lst.Items[0]
		klog.V(log.D).InfoS("Found cluster platform", "platform", res.Name)
		return &res, nil
	}
	klog.V(log.I).InfoS("Not found a cluster platform")
	return nil, nil
}

// listPrimaryClusterPlatforms returns all non-secondary cluster platforms installed (only one will be active).
func listPrimaryClusterPlatforms(ctx context.Context, c ctrl.Reader) (*operatorapi.SonataFlowClusterPlatformList, error) {
	lst, err := listAllClusterPlatforms(ctx, c)
	if err != nil {
		return nil, err
	}

	filtered := &operatorapi.SonataFlowClusterPlatformList{}
	for i := range lst.Items {
		pl := lst.Items[i]
		if !IsSecondary(&pl) {
			filtered.Items = append(filtered.Items, pl)
		}
	}
	return filtered, nil
}

// listAllClusterPlatforms returns all clusterplatforms installed.
func listAllClusterPlatforms(ctx context.Context, c ctrl.Reader) (*operatorapi.SonataFlowClusterPlatformList, error) {
	lst := operatorapi.NewSonataFlowClusterPlatformList()
	if err := c.List(ctx, &lst); err != nil {
		return nil, err
	}
	return &lst, nil
}

// IsActive determines if the given platform is being used.
func IsActive(p *operatorapi.SonataFlowClusterPlatform) bool {
	return !p.Status.IsDuplicated()
}

// IsSecondary determines if the given platform is marked as secondary.
func IsSecondary(p *operatorapi.SonataFlowClusterPlatform) bool {
	if l, ok := p.Annotations[metadata.SecondaryPlatformAnnotation]; ok && l == "true" {
		return true
	}
	return false
}

// IsOperatorHandler Operators matching the annotation operator id are allowed to reconcile.
// For legacy resources that are missing a proper operator id annotation the default global operator or the local
// operator in this namespace are candidates for reconciliation.
func IsOperatorHandler(object ctrl.Object) bool {
	if object == nil {
		return true
	}
	resourceID := utils.GetOperatorIDAnnotation(object)
	operatorID := utils.OperatorID()

	// allow operator with matching id to handle the resource
	if resourceID == operatorID {
		return true
	}

	// check if we are dealing with resource that is missing a proper operator id annotation
	if resourceID == "" {
		// allow default global operator to handle legacy resources (missing proper operator id annotations)
		if operatorID == DefaultPlatformName {
			return true
		}

		// allow local operators to handle legacy resources (missing proper operator id annotations)
		if !IsCurrentOperatorGlobal() {
			return true
		}
	}

	return false
}
