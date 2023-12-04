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

	"github.com/apache/incubator-kie-kogito-serverless-operator/api"
	"github.com/apache/incubator-kie-kogito-serverless-operator/api/metadata"
	operatorapi "github.com/apache/incubator-kie-kogito-serverless-operator/api/v1alpha08"
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

func (action *initializeAction) Handle(ctx context.Context, platform *operatorapi.SonataFlowClusterPlatform) (*operatorapi.SonataFlowClusterPlatform, error) {
	duplicate, err := action.isPrimaryDuplicate(ctx, platform)
	if err != nil {
		return nil, err
	}
	if duplicate {
		// another platform already present in the namespace
		if !platform.Status.IsDuplicated() {
			plat := platform.DeepCopy()
			plat.Status.Manager().MarkFalse(api.SucceedConditionType, operatorapi.PlatformDuplicatedReason, "")
			return plat, nil
		}

		return nil, nil
	}
	platform.Status.Version = metadata.SpecVersion

	return platform, nil
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
