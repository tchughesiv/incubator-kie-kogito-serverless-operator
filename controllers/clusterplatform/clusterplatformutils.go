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

/*
import (
	"runtime"

	operatorapi "github.com/apache/incubator-kie-kogito-serverless-operator/api/v1alpha08"
	"github.com/apache/incubator-kie-kogito-serverless-operator/container-builder/util/defaults"
	"github.com/apache/incubator-kie-kogito-serverless-operator/log"
	"k8s.io/klog/v2"
)

func setStatusAdditionalInfo(platform *operatorapi.SonataFlowClusterPlatform) {
	platform.Status.Info = make(map[string]string)

	klog.V(log.D).InfoS("SonataFlow Platform setting build publish strategy", "namespace", platform.Namespace)
	if platform.Spec.Build.Config.BuildStrategy == operatorapi.OperatorBuildStrategy {
		platform.Status.Info["kanikoVersion"] = defaults.KanikoVersion
	}
	klog.V(log.D).InfoS("SonataFlow setting status info", "namespace", platform.Namespace)
	platform.Status.Info["goVersion"] = runtime.Version()
	platform.Status.Info["goOS"] = runtime.GOOS
}
*/
