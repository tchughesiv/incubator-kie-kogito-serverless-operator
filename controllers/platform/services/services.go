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

package services

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	operatorapi "github.com/apache/incubator-kie-kogito-serverless-operator/api/v1alpha08"
	"github.com/apache/incubator-kie-kogito-serverless-operator/controllers/profiles/common/constants"
	"github.com/magiconair/properties"

	"github.com/apache/incubator-kie-kogito-serverless-operator/version"
	"github.com/imdario/mergo"
)

type Platform interface {
	// GetContainerName returns the name of the service's container in the deployment.
	GetContainerName() string
	// GetServiceImageName returns the image name of the service's container. It takes in the service and persistence types and returns a string
	// that contains the FQDN of the image, including the tag.
	GetServiceImageName(persistenceName string) string
	// GetServiceName returns the name of the kubernetes service prefixed with the platform name
	GetServiceName() string
	// GetServiceCmName returns the name of the configmap associated to the service
	GetServiceCmName() string
	// GetEnvironmentVariables returns the env variables to be injected to the service container
	GetEnvironmentVariables() []corev1.EnvVar
	// GetResourceLimits returns the pod's memory and CPU resource requirements
	// Values for job service taken from
	// https://github.com/parodos-dev/orchestrator-helm-chart/blob/52d09eda56fdbed3060782df29847c97f172600f/charts/orchestrator/values.yaml#L68-L72
	GetPodResourceRequirements() corev1.ResourceRequirements
	// GetReplicaCountForService Returns the default pod replica count for the given service
	GetReplicaCount() int32

	// MergeContainerSpec performs a merge with override using the containerSpec argument and the expected values based on the service's pod template specifications. The returning
	// object is the merged result
	MergeContainerSpec(containerSpec *corev1.Container) (*corev1.Container, error)

	//ConfigurePersistence sets the persistence's image and environment values when it is defined in the Persistence field of the service, overriding any existing value.
	ConfigurePersistence(containerSpec *corev1.Container) *corev1.Container

	//MergePodSpec performs a merge with override between the podSpec argument and the expected values based on the service's pod template specification. The returning
	// object is the result of the merge
	MergePodSpec(podSpec corev1.PodSpec) (corev1.PodSpec, error)
	// GenerateWorkflowProperties returns a property object that contains the service's application properties required by workflows
	GenerateWorkflowProperties() (*properties.Properties, error)
	// GenerateServiceProperties returns a property object that contains the application properties required by the service deployment
	GenerateServiceProperties() (*properties.Properties, error)

	// IsServiceSetInSpec returns true if the service is set in the spec.
	IsServiceSetInSpec() bool
	// IsServiceEnabledInSpec returns true if the service is enabled in the spec.
	IsServiceEnabledInSpec() bool
	// GetLocalServiceBaseUrl returns the base url of the local service
	GetLocalServiceBaseUrl() string
	// GetServiceBaseUrl returns the base url of the service, based on whether using local or cluster-scoped service.
	GetServiceBaseUrl() string
	// GetServiceUrl returns the service url, based on whether using local or cluster-scoped service.
	GetServiceUrl() string
	// IsServiceEnabled returns true if the service is enabled in either the spec or the status.clusterPlatformRef.
	IsServiceEnabled() bool
	// SetServiceUrlInStatus sets the service url in status. if reconciled instance does not have service set in spec AND
	// if cluster referenced platform has said service enabled, use the cluster platform's service
	SetServiceUrlInStatus(clusterRefPlatform *operatorapi.SonataFlowPlatform)
}

type DataIndex struct {
	platform *operatorapi.SonataFlowPlatform
}

func NewDataIndexService(platform *operatorapi.SonataFlowPlatform) Platform {
	return DataIndex{platform: platform}
}

func (d DataIndex) GetContainerName() string {
	return constants.DataIndexServiceName
}

func (d DataIndex) GetServiceImageName(persistenceName string) string {
	var tag = version.GetMajorMinor()
	var suffix = ""
	if version.IsSnapshot() {
		tag = "latest"
		//TODO, remove
		suffix = constants.ImageNameNightlySuffix
	}
	// returns "quay.io/kiegroup/kogito-data-index-<persistence_layer>:<tag>"
	return fmt.Sprintf("%s-%s-%s:%s", constants.ImageNamePrefix, constants.DataIndexName, persistenceName+suffix, tag)
}

func (d DataIndex) GetServiceName() string {
	return fmt.Sprintf("%s-%s", d.platform.Name, constants.DataIndexServiceName)
}

func (d DataIndex) SetServiceUrlInStatus(clusterRefPlatform *operatorapi.SonataFlowPlatform) {
	psDI := NewDataIndexService(clusterRefPlatform)
	if !isServicesSet(d.platform) && psDI.IsServiceEnabledInSpec() {
		if d.platform.Status.ClusterPlatformRef != nil {
			if d.platform.Status.ClusterPlatformRef.Services == nil {
				d.platform.Status.ClusterPlatformRef.Services = &operatorapi.PlatformServices{}
			}
			d.platform.Status.ClusterPlatformRef.Services.DataIndexRef = &operatorapi.PlatformServiceRef{
				Url: psDI.GetLocalServiceBaseUrl(),
			}
		}
	}
}

func (d DataIndex) IsServiceSetInSpec() bool {
	return isDataIndexSet(d.platform)
}

func (d DataIndex) IsServiceEnabledInSpec() bool {
	return isDataIndexEnabled(d.platform)
}

func (d DataIndex) isServiceEnabledInStatus() bool {
	return d.platform != nil && d.platform.Status.ClusterPlatformRef != nil &&
		d.platform.Status.ClusterPlatformRef.Services != nil && d.platform.Status.ClusterPlatformRef.Services.DataIndexRef != nil &&
		!isServicesSet(d.platform)
}

func (d DataIndex) IsServiceEnabled() bool {
	return d.IsServiceEnabledInSpec() || d.isServiceEnabledInStatus()
}

func (d DataIndex) GetServiceUrl() string {
	return d.GetServiceBaseUrl() + constants.DataIndexServiceURLPath
}

func (d DataIndex) GetServiceBaseUrl() string {
	if d.IsServiceEnabledInSpec() {
		return d.GetLocalServiceBaseUrl()
	}
	if d.isServiceEnabledInStatus() {
		return d.platform.Status.ClusterPlatformRef.Services.DataIndexRef.Url
	}
	return ""
}

func (d DataIndex) GetLocalServiceBaseUrl() string {
	return fmt.Sprintf("%s://%s.%s", constants.DataIndexServiceURLProtocol, d.GetServiceName(), d.platform.Namespace)
}

func (d DataIndex) GetEnvironmentVariables() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KOGITO_DATA_INDEX_QUARKUS_PROFILE",
			Value: "http-events-support",
		},
		{
			Name:  "QUARKUS_HTTP_CORS",
			Value: "true",
		},
		{
			Name:  "QUARKUS_HTTP_CORS_ORIGINS",
			Value: "/.*/",
		},
	}
}

func (d DataIndex) GetPodResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}

func (d DataIndex) MergePodSpec(podSpec corev1.PodSpec) (corev1.PodSpec, error) {
	c := podSpec.DeepCopy()
	err := mergo.Merge(c, d.platform.Spec.Services.DataIndex.PodTemplate.PodSpec.ToPodSpec(), mergo.WithOverride)
	return *c, err
}

func (d DataIndex) ConfigurePersistence(containerSpec *corev1.Container) *corev1.Container {

	if d.platform.Spec.Services.DataIndex.Persistence != nil && d.platform.Spec.Services.DataIndex.Persistence.PostgreSql != nil {
		c := containerSpec.DeepCopy()
		c.Image = d.GetServiceImageName(constants.PersistenceTypePostgreSQL)
		c.Env = append(c.Env, d.configurePostgreSqlEnv(d.platform.Spec.Services.DataIndex.Persistence.PostgreSql, d.GetServiceName(), d.platform.Namespace)...)
		return c
	}
	return containerSpec
}

func (d DataIndex) MergeContainerSpec(containerSpec *corev1.Container) (*corev1.Container, error) {
	c := containerSpec.DeepCopy()
	err := mergo.Merge(c, d.platform.Spec.Services.DataIndex.PodTemplate.Container.ToContainer(), mergo.WithOverride)
	return c, err
}

func (d DataIndex) GetReplicaCount() int32 {
	if d.platform.Spec.Services.DataIndex.PodTemplate.Replicas != nil {
		return *d.platform.Spec.Services.DataIndex.PodTemplate.Replicas
	}
	return 1
}

func (d DataIndex) GetServiceCmName() string {
	return fmt.Sprintf("%s-props", d.GetServiceName())
}

func (d DataIndex) configurePostgreSqlEnv(postgresql *operatorapi.PersistencePostgreSql, databaseSchema, databaseNamespace string) []corev1.EnvVar {
	dataSourcePort := constants.DefaultPostgreSQLPort
	databaseName := "sonataflow"
	dataSourceURL := postgresql.JdbcUrl
	if postgresql.ServiceRef != nil {
		if len(postgresql.ServiceRef.DatabaseSchema) > 0 {
			databaseSchema = postgresql.ServiceRef.DatabaseSchema
		}
		if len(postgresql.ServiceRef.Namespace) > 0 {
			databaseNamespace = postgresql.ServiceRef.Namespace
		}
		if postgresql.ServiceRef.Port != nil {
			dataSourcePort = *postgresql.ServiceRef.Port
		}
		if len(postgresql.ServiceRef.DatabaseName) > 0 {
			databaseName = postgresql.ServiceRef.DatabaseName
		}
		dataSourceURL = "jdbc:" + constants.PersistenceTypePostgreSQL + "://" + postgresql.ServiceRef.Name + "." + databaseNamespace + ":" + strconv.Itoa(dataSourcePort) + "/" + databaseName + "?currentSchema=" + databaseSchema
	}
	secretRef := corev1.LocalObjectReference{
		Name: postgresql.SecretRef.Name,
	}
	quarkusDatasourceUsername := "POSTGRESQL_USER"
	if len(postgresql.SecretRef.UserKey) > 0 {
		quarkusDatasourceUsername = postgresql.SecretRef.UserKey
	}
	quarkusDatasourcePassword := "POSTGRESQL_PASSWORD"
	if len(postgresql.SecretRef.PasswordKey) > 0 {
		quarkusDatasourcePassword = postgresql.SecretRef.PasswordKey
	}
	return []corev1.EnvVar{
		{
			Name: "QUARKUS_DATASOURCE_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  quarkusDatasourceUsername,
					LocalObjectReference: secretRef,
				},
			},
		},
		{
			Name: "QUARKUS_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  quarkusDatasourcePassword,
					LocalObjectReference: secretRef,
				},
			},
		},
		{
			Name:  "QUARKUS_DATASOURCE_DB_KIND",
			Value: constants.PersistenceTypePostgreSQL,
		},
		{
			Name:  "QUARKUS_HIBERNATE_ORM_DATABASE_GENERATION",
			Value: "update",
		},
		{
			Name:  "QUARKUS_FLYWAY_MIGRATE_AT_START",
			Value: "true",
		},
		{
			Name:  "QUARKUS_DATASOURCE_JDBC_URL",
			Value: dataSourceURL,
		},
	}
}

func (d DataIndex) GenerateWorkflowProperties() (*properties.Properties, error) {
	props := properties.NewProperties()
	if d.IsServiceEnabled() {
		props.Set(constants.DataIndexServiceURLProperty, d.GetServiceUrl())
	}
	return props, nil
}

func (d DataIndex) GenerateServiceProperties() (*properties.Properties, error) {
	props := properties.NewProperties()
	props.Set(constants.DataIndexKafkaSmallRyeHealthProperty, "false")
	return props, nil
}

type JobService struct {
	platform *operatorapi.SonataFlowPlatform
}

func NewJobService(platform *operatorapi.SonataFlowPlatform) Platform {
	return JobService{platform: platform}
}

func (j JobService) GetContainerName() string {
	return constants.JobServiceName
}

func (j JobService) GetServiceImageName(persistenceName string) string {
	var tag = version.GetMajorMinor()
	var suffix = ""
	if version.IsSnapshot() {
		tag = "latest"
		//TODO remove
		suffix = constants.ImageNameNightlySuffix
	}
	// returns "quay.io/kiegroup/kogito-jobs-service-<persistece_layer>:<tag>"
	return fmt.Sprintf("%s-%s-%s:%s", constants.ImageNamePrefix, constants.JobServiceName, persistenceName+suffix, tag)
}

func (j JobService) GetServiceName() string {
	return fmt.Sprintf("%s-%s", j.platform.Name, constants.JobServiceName)
}

func (j JobService) GetServiceCmName() string {
	return fmt.Sprintf("%s-props", j.GetServiceName())
}

func (j JobService) SetServiceUrlInStatus(clusterRefPlatform *operatorapi.SonataFlowPlatform) {
	psJS := NewJobService(clusterRefPlatform)
	if !isServicesSet(j.platform) && psJS.IsServiceEnabledInSpec() {
		if j.platform.Status.ClusterPlatformRef != nil {
			if j.platform.Status.ClusterPlatformRef.Services == nil {
				j.platform.Status.ClusterPlatformRef.Services = &operatorapi.PlatformServices{}
			}
			j.platform.Status.ClusterPlatformRef.Services.JobServiceRef = &operatorapi.PlatformServiceRef{
				Url: psJS.GetLocalServiceBaseUrl(),
			}
		}
	}
}

func (j JobService) IsServiceSetInSpec() bool {
	return isJobServiceSet(j.platform)
}

func (j JobService) IsServiceEnabledInSpec() bool {
	return isJobServiceEnabled(j.platform)
}

func (j JobService) isServiceEnabledInStatus() bool {
	return j.platform != nil && j.platform.Status.ClusterPlatformRef != nil &&
		j.platform.Status.ClusterPlatformRef.Services != nil && j.platform.Status.ClusterPlatformRef.Services.JobServiceRef != nil &&
		!isServicesSet(j.platform)
}

func (j JobService) IsServiceEnabled() bool {
	return j.IsServiceEnabledInSpec() || j.isServiceEnabledInStatus()
}

func (j JobService) GetServiceUrl() string {
	return j.GetServiceBaseUrl() + constants.JobServiceURLPath
}

func (j JobService) GetServiceBaseUrl() string {
	if j.IsServiceEnabledInSpec() {
		return j.GetLocalServiceBaseUrl()
	}
	if j.isServiceEnabledInStatus() {
		return j.platform.Status.ClusterPlatformRef.Services.JobServiceRef.Url
	}
	return ""
}

func (j JobService) GetLocalServiceBaseUrl() string {
	return fmt.Sprintf("%s://%s.%s", constants.JobServiceURLProtocol, j.GetServiceName(), j.platform.Namespace)
}

func (j JobService) GetEnvironmentVariables() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "QUARKUS_HTTP_CORS",
			Value: "true",
		},
		{
			Name:  "QUARKUS_HTTP_CORS_ORIGINS",
			Value: "/.*/",
		},
	}
}

func (j JobService) GetPodResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("250m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
}

func (j JobService) GetReplicaCount() int32 {
	return 1
}

func (j JobService) MergeContainerSpec(containerSpec *corev1.Container) (*corev1.Container, error) {
	c := containerSpec.DeepCopy()
	err := mergo.Merge(c, j.platform.Spec.Services.JobService.PodTemplate.Container.ToContainer(), mergo.WithOverride)
	return c, err
}

func (j JobService) ConfigurePersistence(containerSpec *corev1.Container) *corev1.Container {

	if j.platform.Spec.Services.JobService.Persistence != nil && j.platform.Spec.Services.JobService.Persistence.PostgreSql != nil {
		c := containerSpec.DeepCopy()
		c.Image = j.GetServiceImageName(constants.PersistenceTypePostgreSQL)
		c.Env = append(c.Env, j.configurePostgreSqlEnv(j.platform.Spec.Services.JobService.Persistence.PostgreSql, j.GetServiceName(), j.platform.Namespace)...)
		return c
	}
	return containerSpec
}

func (j JobService) MergePodSpec(podSpec corev1.PodSpec) (corev1.PodSpec, error) {
	c := podSpec.DeepCopy()
	err := mergo.Merge(c, j.platform.Spec.Services.JobService.PodTemplate.PodSpec.ToPodSpec(), mergo.WithOverride)
	return *c, err
}

func (j JobService) configurePostgreSqlEnv(postgresql *operatorapi.PersistencePostgreSql, databaseSchema, databaseNamespace string) []corev1.EnvVar {
	dataSourcePort := constants.DefaultPostgreSQLPort
	databaseName := "sonataflow"
	dataSourceURL := postgresql.JdbcUrl
	if postgresql.ServiceRef != nil {
		if len(postgresql.ServiceRef.DatabaseSchema) > 0 {
			databaseSchema = postgresql.ServiceRef.DatabaseSchema
		}
		if len(postgresql.ServiceRef.Namespace) > 0 {
			databaseNamespace = postgresql.ServiceRef.Namespace
		}
		if postgresql.ServiceRef.Port != nil {
			dataSourcePort = *postgresql.ServiceRef.Port
		}
		if len(postgresql.ServiceRef.DatabaseName) > 0 {
			databaseName = postgresql.ServiceRef.DatabaseName
		}
		dataSourceURL = "jdbc:" + constants.PersistenceTypePostgreSQL + "://" + postgresql.ServiceRef.Name + "." + databaseNamespace + ":" + strconv.Itoa(dataSourcePort) + "/" + databaseName + "?currentSchema=" + databaseSchema
	}

	secretRef := corev1.LocalObjectReference{
		Name: postgresql.SecretRef.Name,
	}
	quarkusDatasourceUsername := "POSTGRESQL_USER"
	if len(postgresql.SecretRef.UserKey) > 0 {
		quarkusDatasourceUsername = postgresql.SecretRef.UserKey
	}
	quarkusDatasourcePassword := "POSTGRESQL_PASSWORD"
	if len(postgresql.SecretRef.PasswordKey) > 0 {
		quarkusDatasourcePassword = postgresql.SecretRef.PasswordKey
	}
	return []corev1.EnvVar{
		{
			Name: "QUARKUS_DATASOURCE_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  quarkusDatasourceUsername,
					LocalObjectReference: secretRef,
				},
			},
		},
		{
			Name: "QUARKUS_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key:                  quarkusDatasourcePassword,
					LocalObjectReference: secretRef,
				},
			},
		},
		{
			Name:  "QUARKUS_DATASOURCE_DB_KIND",
			Value: constants.PersistenceTypePostgreSQL,
		},
		{
			Name:  "QUARKUS_FLYWAY_MIGRATE_AT_START",
			Value: "true",
		},
		{
			Name:  "QUARKUS_DATASOURCE_JDBC_URL",
			Value: dataSourceURL,
		},
	}
}

func (j JobService) GenerateServiceProperties() (*properties.Properties, error) {
	props := properties.NewProperties()
	props.Set(constants.JobServiceKafkaSmallRyeHealthProperty, "false")
	// add data source reactive URL
	jspec := j.platform.Spec.Services.JobService
	if j.IsServiceSetInSpec() && jspec.Persistence != nil && jspec.Persistence.PostgreSql != nil {
		dataSourceReactiveURL, err := generateReactiveURL(jspec.Persistence.PostgreSql, j.GetServiceName(), j.platform.Namespace, constants.DefaultDatabaseName, constants.DefaultPostgreSQLPort)
		if err != nil {
			return nil, err
		}
		props.Set(constants.JobServiceDataSourceReactiveURL, dataSourceReactiveURL)
	}
	di := NewDataIndexService(j.platform)
	if di.IsServiceEnabled() {
		props.Set(constants.JobServiceStatusChangeEvents, "true")
		props.Set(constants.JobServiceStatusChangeEventsURL, di.GetServiceBaseUrl()+"/jobs")
	}
	props.Sort()
	return props, nil
}

func (j JobService) GenerateWorkflowProperties() (*properties.Properties, error) {
	props := properties.NewProperties()
	if j.IsServiceEnabled() {
		// add data source reactive URL
		props.Set(constants.JobServiceRequestEventsURL, j.GetServiceUrl())
	}
	return props, nil
}

func isDataIndexEnabled(platform *operatorapi.SonataFlowPlatform) bool {
	return isDataIndexSet(platform) && platform.Spec.Services.DataIndex.Enabled != nil &&
		*platform.Spec.Services.DataIndex.Enabled
}

func isJobServiceEnabled(platform *operatorapi.SonataFlowPlatform) bool {
	return isJobServiceSet(platform) && platform.Spec.Services.JobService.Enabled != nil &&
		*platform.Spec.Services.JobService.Enabled
}

func isDataIndexSet(platform *operatorapi.SonataFlowPlatform) bool {
	return isServicesSet(platform) && platform.Spec.Services.DataIndex != nil
}

func isJobServiceSet(platform *operatorapi.SonataFlowPlatform) bool {
	return isServicesSet(platform) && platform.Spec.Services.JobService != nil
}

func isServicesSet(platform *operatorapi.SonataFlowPlatform) bool {
	return platform != nil && platform.Spec.Services != nil
}
