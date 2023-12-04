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
	operatorapi "github.com/apache/incubator-kie-kogito-serverless-operator/api/v1alpha08"
	"github.com/apache/incubator-kie-kogito-serverless-operator/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewWarmAction(reader ctrl.Reader) Action {
	return &warmAction{
		reader: reader,
	}
}

type warmAction struct {
	baseAction
	reader ctrl.Reader
}

func (action *warmAction) Name() string {
	return "warm"
}

func (action *warmAction) CanHandle(platform *operatorapi.SonataFlowClusterPlatform) bool {
	return platform.Status.IsWarming()
}

func (action *warmAction) Handle(ctx context.Context, cPlatform *operatorapi.SonataFlowClusterPlatform) (*operatorapi.SonataFlowClusterPlatform, error) {
	platformRef := cPlatform.Spec.PlatformRef

	// Check platform status
	platform := operatorapi.SonataFlowPlatform{}
	err := action.reader.Get(ctx, types.NamespacedName{Namespace: platformRef.Namespace, Name: platformRef.Name}, &platform)
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
		klog.V(log.D).InfoS("Kaniko cache successfully warmed up")
		cPlatform.Status.Manager().MarkTrueWithReason(api.SucceedConditionType, operatorapi.PlatformWarmingReason, "Kaniko cache successfully warmed up")
		return cPlatform, nil
		//	case api.RunningConditionType:
		//		return nil, errors.New("failed to warm up Kaniko cache")
	default:
		klog.V(log.I).InfoS("Waiting for Kaniko cache to warm up...")
		// Requeue
		return nil, nil
	}
}
