// Copyright 2018 Google Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/container-structure-test/pkg/types/unversioned"
)

func resultsChannel(results ...*unversioned.TestResult) chan interface{} {
	c := make(chan interface{}, len(results))
	for _, r := range results {
		c <- r
	}
	close(c)
	return c
}

// ProcessResults must write a human-readable summary to the primary writer
// (text format) while writing a machine-readable report to reportOut, so a
// report file no longer silences stdout.
func TestProcessResultsDualOutput(t *testing.T) {
	var stdout, report bytes.Buffer
	c := resultsChannel(&unversioned.TestResult{Name: "passing", Pass: true})

	if err := ProcessResults(&stdout, &report, unversioned.Text, unversioned.Json, "", c); err != nil {
		t.Fatalf("ProcessResults returned error: %v", err)
	}

	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "RESULTS") || !strings.Contains(stdoutStr, "PASS") {
		t.Errorf("primary stdout missing human-readable summary, got:\n%s", stdoutStr)
	}
	if strings.HasPrefix(strings.TrimSpace(stdoutStr), "{") {
		t.Errorf("primary stdout should be text, not JSON, got:\n%s", stdoutStr)
	}

	var summary unversioned.SummaryObject
	if err := json.Unmarshal(report.Bytes(), &summary); err != nil {
		t.Fatalf("report output is not valid JSON: %v\ngot:\n%s", err, report.String())
	}
	if summary.Pass != 1 || summary.Total != 1 {
		t.Errorf("report summary = %+v, want Pass=1 Total=1", summary)
	}
}

// When no report writer is supplied, only the primary writer is used and the
// summary still reflects the results.
func TestProcessResultsNoReport(t *testing.T) {
	var stdout bytes.Buffer
	c := resultsChannel(
		&unversioned.TestResult{Name: "passing", Pass: true},
		&unversioned.TestResult{Name: "failing", Pass: false},
	)

	err := ProcessResults(&stdout, nil, unversioned.Json, unversioned.Json, "", c)
	if err == nil {
		t.Fatal("expected non-nil error when a test fails")
	}

	var summary unversioned.SummaryObject
	if jsonErr := json.Unmarshal(stdout.Bytes(), &summary); jsonErr != nil {
		t.Fatalf("primary output is not valid JSON: %v\ngot:\n%s", jsonErr, stdout.String())
	}
	if summary.Pass != 1 || summary.Fail != 1 || summary.Total != 2 {
		t.Errorf("summary = %+v, want Pass=1 Fail=1 Total=2", summary)
	}
}
