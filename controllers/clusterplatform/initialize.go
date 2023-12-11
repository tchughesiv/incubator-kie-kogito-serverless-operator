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

	"github.com/apache/incubator-kie-kogito-serverless-operator/api"
	"github.com/apache/incubator-kie-kogito-serverless-operator/api/metadata"
	operatorapi "github.com/apache/incubator-kie-kogito-serverless-operator/api/v1alpha08"
	"github.com/apache/incubator-kie-kogito-serverless-operator/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// NewInitializeAction returns an action that initializes the platform configuration when not provided by the user.
func NewInitializeAction() Action {
	return &initializeAction{}
}

type initializeAction struct {
	baseAction
}

func (action *initializeAction) Name() string {
	return "initialize"
}

func (action *initializeAction) CanHandle(platform *operatorapi.SonataFlowClusterPlatform) bool {
	return platform.Status.GetTopLevelCondition().IsUnknown() || platform.Status.IsDuplicated()
}

func (action *initializeAction) Handle(ctx context.Context, cPlatform *operatorapi.SonataFlowClusterPlatform) (*operatorapi.SonataFlowClusterPlatform, error) {
	duplicate, err := action.isPrimaryDuplicate(ctx, cPlatform)
	if err != nil {
		return nil, err
	}
	if duplicate {
		// another platform already present in the namespace
		if !cPlatform.Status.IsDuplicated() {
			cPlat := cPlatform.DeepCopy()
			cPlat.Status.Manager().MarkFalse(api.SucceedConditionType, operatorapi.PlatformDuplicatedReason, "")
			return cPlat, nil
		}

		return nil, nil
	}
	cPlatform.Status.Version = metadata.SpecVersion
	platformRef := cPlatform.Spec.PlatformRef

	// Check platform status
	platform := operatorapi.SonataFlowPlatform{}
	err = action.client.Get(ctx, types.NamespacedName{Namespace: platformRef.Namespace, Name: platformRef.Name}, &platform)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.V(log.D).InfoS("%s platform does not exist in %s namespace.", platformRef.Name, platformRef.Namespace)
			cPlatform.Status.Manager().MarkFalse(api.FailedConditionType, operatorapi.PlatformNotFoundReason,
				fmt.Sprintf("%s platform does not exist in %s namespace.", platformRef.Name, platformRef.Namespace))
			return cPlatform, nil
		}
		return nil, err
	}

	switch platform.Status.GetTopLevelCondition().Type {
	case api.SucceedConditionType:
		klog.V(log.D).InfoS("Cluster platform successfully warmed up")
		cPlatform.Status.Manager().MarkTrueWithReason(api.SucceedConditionType, operatorapi.PlatformWarmingReason, "Cluster platform successfully warmed up")
		return cPlatform, nil
		//	case api.RunningConditionType:
		//		return nil, errors.New("failed to warm up Kaniko cache")
	default:
		klog.V(log.D).InfoS("Waiting for %s platform to warm up in %s namespace...", platformRef.Name, platformRef.Namespace)
		// Requeue
		return nil, nil
	}

	//return cPlatform, nil
}

// Function to double-check if there is already an active platform on the current context (i.e. namespace)
func (action *initializeAction) isPrimaryDuplicate(ctx context.Context, thisPlatform *operatorapi.SonataFlowClusterPlatform) (bool, error) {
	if IsSecondary(thisPlatform) {
		// Always reconcile secondary platforms
		return false, nil
	}
	platforms, err := listPrimaryClusterPlatforms(ctx, action.client)
	if err != nil {
		return false, err
	}
	for _, p := range platforms.Items {
		p := p // pin
		if p.Name != thisPlatform.Name && IsActive(&p) {
			return true, nil
		}
	}

	return false, nil
}
