// Copyright 2017 Google Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"os"
	"testing"

	types "github.com/GoogleContainerTools/container-structure-test/pkg/types/unversioned"
)

// mockDriver provides a mock implementation of drivers.Driver for testing metadata tests
type mockDriver struct {
	config *types.Config
}

func (m *mockDriver) Setup(envVars []types.EnvVar, fullCommands [][]string) error {
	return nil
}

func (m *mockDriver) Teardown(fullCommands [][]string) error {
	return nil
}

func (m *mockDriver) SetEnv(envVars []types.EnvVar) error {
	return nil
}

func (m *mockDriver) ProcessCommand(envVars []types.EnvVar, fullCommand []string) (string, string, int, error) {
	return "", "", 0, nil
}

func (m *mockDriver) GetConfig() (types.Config, error) {
	return *m.config, nil
}

func (m *mockDriver) ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func (m *mockDriver) StatFile(path string) (os.FileInfo, error) {
	return nil, nil
}

func (m *mockDriver) ReadDir(path string) ([]os.FileInfo, error) {
	return nil, nil
}

func (m *mockDriver) Destroy() {
}

func TestExposedPortsWithProtocol(t *testing.T) {
	tests := []struct {
		name              string
		imageExposedPorts []string
		testExposedPorts  []string
		shouldPass        bool
	}{
		{
			name:              "exact match tcp port",
			imageExposedPorts: []string{"53/tcp"},
			testExposedPorts:  []string{"53/tcp"},
			shouldPass:        true,
		},
		{
			name:              "exact match udp port",
			imageExposedPorts: []string{"53/udp"},
			testExposedPorts:  []string{"53/udp"},
			shouldPass:        true,
		},
		{
			name:              "bare port matches tcp",
			imageExposedPorts: []string{"53/tcp"},
			testExposedPorts:  []string{"53"},
			shouldPass:        true,
		},
		{
			name:              "bare port matches udp",
			imageExposedPorts: []string{"53/udp"},
			testExposedPorts:  []string{"53"},
			shouldPass:        true,
		},
		{
			name:              "protocol mismatch fails",
			imageExposedPorts: []string{"53/tcp"},
			testExposedPorts:  []string{"53/udp"},
			shouldPass:        false,
		},
		{
			name:              "multiple ports with protocols",
			imageExposedPorts: []string{"53/tcp", "53/udp", "8080/tcp"},
			testExposedPorts:  []string{"53/tcp", "53/udp"},
			shouldPass:        true,
		},
		{
			name:              "bare port with multiple exposed protocols",
			imageExposedPorts: []string{"53/tcp", "53/udp"},
			testExposedPorts:  []string{"53"},
			shouldPass:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDrv := &mockDriver{
				config: &types.Config{
					Env:          map[string]string{},
					ExposedPorts: tt.imageExposedPorts,
					Labels:       map[string]string{},
				},
			}

			metadataTest := MetadataTest{
				ExposedPorts: tt.testExposedPorts,
			}

			result := metadataTest.Run(mockDrv)

			if tt.shouldPass && !result.IsPass() {
				t.Errorf("expected test to pass but got errors: %v", result.Errors)
			}
			if !tt.shouldPass && result.IsPass() {
				t.Errorf("expected test to fail but it passed")
			}
		})
	}
}

func TestUnexposedPortsWithProtocol(t *testing.T) {
	tests := []struct {
		name               string
		imageExposedPorts  []string
		testUnexposedPorts []string
		shouldPass         bool
	}{
		{
			name:               "port not exposed with exact protocol",
			imageExposedPorts:  []string{"8080/tcp", "9000/tcp"},
			testUnexposedPorts: []string{"53/tcp"},
			shouldPass:         true,
		},
		{
			name:               "bare port not exposed",
			imageExposedPorts:  []string{"8080/tcp", "9000/tcp"},
			testUnexposedPorts: []string{"53"},
			shouldPass:         true,
		},
		{
			name:               "bare port exposed fails test",
			imageExposedPorts:  []string{"53/tcp", "8080/tcp"},
			testUnexposedPorts: []string{"53"},
			shouldPass:         false,
		},
		{
			name:               "exact protocol port exposed fails test",
			imageExposedPorts:  []string{"53/tcp", "8080/tcp"},
			testUnexposedPorts: []string{"53/tcp"},
			shouldPass:         false,
		},
		{
			name:               "protocol mismatch passes unexposed test",
			imageExposedPorts:  []string{"53/tcp", "8080/tcp"},
			testUnexposedPorts: []string{"53/udp"},
			shouldPass:         true,
		},
		{
			name:               "multiple unexposed ports when only some exposed",
			imageExposedPorts:  []string{"53/tcp", "8080/tcp"},
			testUnexposedPorts: []string{"53/udp", "9000/tcp"},
			shouldPass:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDrv := &mockDriver{
				config: &types.Config{
					Env:          map[string]string{},
					ExposedPorts: tt.imageExposedPorts,
					Labels:       map[string]string{},
				},
			}

			metadataTest := MetadataTest{
				UnexposedPorts: tt.testUnexposedPorts,
			}

			result := metadataTest.Run(mockDrv)

			if tt.shouldPass && !result.IsPass() {
				t.Errorf("expected test to pass but got errors: %v", result.Errors)
			}
			if !tt.shouldPass && result.IsPass() {
				t.Errorf("expected test to fail but it passed")
			}
		})
	}
}
