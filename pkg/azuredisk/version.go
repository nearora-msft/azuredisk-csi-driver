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

package azuredisk

import (
	"fmt"
	"runtime"
	"strings"

	"sigs.k8s.io/yaml"
)

// These are set during build time via -ldflags
var (
	driverVersion = "N/A"
	gitCommit     = "N/A"
	buildDate     = "N/A"
	topologyKey   = "N/A"
)

// VersionInfo holds the version information of the driver
type VersionInfo struct {
	DriverName    string `json:"Driver Name"`
	DriverVersion string `json:"Driver Version"`
	GitCommit     string `json:"Git Commit"`
	BuildDate     string `json:"Build Date"`
	GoVersion     string `json:"Go Version"`
	Compiler      string `json:"Compiler"`
	Platform      string `json:"Platform"`
	TopologyKey   string `json:"Topology Key"`
}

// GetVersion returns the version information of the driver
func GetVersion(driverName string) VersionInfo {
	return VersionInfo{
		DriverName:    driverName,
		DriverVersion: driverVersion,
		GitCommit:     gitCommit,
		BuildDate:     buildDate,
		GoVersion:     runtime.Version(),
		Compiler:      runtime.Compiler,
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		TopologyKey:   topologyKey,
	}
}

// GetVersionYAML returns the version information of the driver
// in YAML format
func GetVersionYAML(driverName string) (string, error) {
	info := GetVersion(driverName)
	marshalled, err := yaml.Marshal(&info)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(marshalled)), nil
}

// GetUserAgent returns user agent of the driver
func GetUserAgent(driverName, customUserAgent, userAgentSuffix string) string {
	customUserAgent = strings.TrimSpace(customUserAgent)
	userAgent := customUserAgent
	if customUserAgent == "" {
		userAgent = fmt.Sprintf("%s/%s", driverName, driverVersion)
	}

	userAgentSuffix = strings.TrimSpace(userAgentSuffix)
	if userAgentSuffix != "" {
		userAgent = userAgent + " " + userAgentSuffix
	}
	return userAgent
}
