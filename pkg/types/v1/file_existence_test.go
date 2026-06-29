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

package v1

import "testing"

func TestFileExistenceTestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		test      FileExistenceTest
		expectErr bool
	}{
		{
			name:      "missing name",
			test:      FileExistenceTest{Path: "/foo"},
			expectErr: true,
		},
		{
			name:      "missing path",
			test:      FileExistenceTest{Name: "no path"},
			expectErr: true,
		},
		{
			name:      "valid",
			test:      FileExistenceTest{Name: "ok", Path: "/foo"},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.test.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("expected validation error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
