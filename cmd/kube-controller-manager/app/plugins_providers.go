//go:build !providerless
// +build !providerless

/*
Copyright 2019 The Kubernetes Authors.

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

package app

import (
	"k8s.io/component-base/featuregate"
	"k8s.io/csi-translation-lib/plugins"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/awsebs"
	"k8s.io/kubernetes/pkg/volume/azure_file"
	"k8s.io/kubernetes/pkg/volume/azuredd"
	"k8s.io/kubernetes/pkg/volume/cinder"
	"k8s.io/kubernetes/pkg/volume/csimigration"
	"k8s.io/kubernetes/pkg/volume/gcepd"
	"k8s.io/kubernetes/pkg/volume/rbd"
	"k8s.io/kubernetes/pkg/volume/vsphere_volume"
)

type probeFn func() []volume.VolumePlugin

func appendPluginBasedOnFeatureFlags(plugins []volume.VolumePlugin, inTreePluginName string, featureGate featuregate.FeatureGate, pluginInfo pluginInfo) ([]volume.VolumePlugin, error) {

	_, err := csimigration.CheckMigrationFeatureFlags(featureGate, pluginInfo.pluginMigrationFeature, pluginInfo.pluginUnregisterFeature)
	if err != nil {
		klog.Warningf("Unexpected CSI Migration Feature Flags combination detected: %v. CSI Migration may not take effect", err)
		// TODO: fail and return here once alpha only tests can set the feature flags for a plugin correctly
	}

	// Skip appending the in-tree plugin to the list of plugins to be probed/initialized
	// if the plugin unregister feature flag is set
	if featureGate.Enabled(pluginInfo.pluginUnregisterFeature) {
		klog.Infof("Skip registration of plugin %s since feature flag %v is enabled", inTreePluginName, pluginInfo.pluginUnregisterFeature)
		return plugins, nil
	}
	plugins = append(plugins, pluginInfo.pluginProbeFunction()...)
	return plugins, nil
}

type pluginInfo struct {
	pluginMigrationFeature  featuregate.Feature
	pluginUnregisterFeature featuregate.Feature
	pluginProbeFunction     probeFn
}

func appendAttachableLegacyProviderVolumes(allPlugins []volume.VolumePlugin, featureGate featuregate.FeatureGate) ([]volume.VolumePlugin, error) {
	pluginMigrationStatus := make(map[string]pluginInfo)
	pluginMigrationStatus[plugins.AWSEBSInTreePluginName] = pluginInfo{pluginMigrationFeature: features.CSIMigrationAWS, pluginUnregisterFeature: features.InTreePluginAWSUnregister, pluginProbeFunction: awsebs.ProbeVolumePlugins}
	pluginMigrationStatus[plugins.GCEPDInTreePluginName] = pluginInfo{pluginMigrationFeature: features.CSIMigrationGCE, pluginUnregisterFeature: features.InTreePluginGCEUnregister, pluginProbeFunction: gcepd.ProbeVolumePlugins}
	pluginMigrationStatus[plugins.CinderInTreePluginName] = pluginInfo{pluginMigrationFeature: features.CSIMigrationOpenStack, pluginUnregisterFeature: features.InTreePluginOpenStackUnregister, pluginProbeFunction: cinder.ProbeVolumePlugins}
	pluginMigrationStatus[plugins.AzureDiskInTreePluginName] = pluginInfo{pluginMigrationFeature: features.CSIMigrationAzureDisk, pluginUnregisterFeature: features.InTreePluginAzureDiskUnregister, pluginProbeFunction: azuredd.ProbeVolumePlugins}
	pluginMigrationStatus[plugins.VSphereInTreePluginName] = pluginInfo{pluginMigrationFeature: features.CSIMigrationvSphere, pluginUnregisterFeature: features.InTreePluginvSphereUnregister, pluginProbeFunction: vsphere_volume.ProbeVolumePlugins}
	pluginMigrationStatus[plugins.RBDVolumePluginName] = pluginInfo{pluginMigrationFeature: features.CSIMigrationRBD, pluginUnregisterFeature: features.InTreePluginRBDUnregister, pluginProbeFunction: rbd.ProbeVolumePlugins}
	var err error
	for pluginName, pluginInfo := range pluginMigrationStatus {
		allPlugins, err = appendPluginBasedOnFeatureFlags(allPlugins, pluginName, featureGate, pluginInfo)
		if err != nil {
			return allPlugins, err
		}
	}
	return allPlugins, nil
}

func appendExpandableLegacyProviderVolumes(allPlugins []volume.VolumePlugin, featureGate featuregate.FeatureGate) ([]volume.VolumePlugin, error) {
	return appendLegacyProviderVolumes(allPlugins, featureGate)
}

func appendLegacyProviderVolumes(allPlugins []volume.VolumePlugin, featureGate featuregate.FeatureGate) ([]volume.VolumePlugin, error) {
	var err error
	// First append attachable volumes
	allPlugins, err = appendAttachableLegacyProviderVolumes(allPlugins, featureGate)
	if err != nil {
		return allPlugins, err
	}

	// Then append non-attachable volumes
	pluginName := plugins.AzureFileInTreePluginName
	pluginInfo := pluginInfo{
		pluginMigrationFeature:  features.CSIMigrationAzureFile,
		pluginUnregisterFeature: features.InTreePluginAzureFileUnregister,
		pluginProbeFunction:     azure_file.ProbeVolumePlugins,
	}
	allPlugins, err = appendPluginBasedOnFeatureFlags(allPlugins, pluginName, featureGate, pluginInfo)
	if err != nil {
		return allPlugins, err
	}

	return allPlugins, nil
}
