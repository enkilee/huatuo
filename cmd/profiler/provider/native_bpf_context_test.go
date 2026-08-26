// Copyright 2026 The HuaTuo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"errors"
	"testing"

	"huatuo-bamai/internal/bpf"
)

type closeBPFStub struct {
	bpf.BPF
	closeErr error
	closed   bool
}

func (s *closeBPFStub) Close() error {
	s.closed = true
	return s.closeErr
}

func TestValidateStackID(t *testing.T) {
	tests := []struct {
		name    string
		stackID int32
		want    bool
	}{
		{name: "negative ID", stackID: -1, want: false},
		{name: "zero ID", stackID: 0, want: true},
		{name: "positive ID", stackID: 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateStackID(tt.stackID); got != tt.want {
				t.Fatalf("validateStackID(%d) = %t, want %t", tt.stackID, got, tt.want)
			}
		})
	}
}

func TestCloseBPF(t *testing.T) {
	t.Run("nil BPF", func(t *testing.T) {
		if err := closeBPF(nil); err != nil {
			t.Fatalf("closeBPF(nil) error = %v, want nil", err)
		}
	})

	t.Run("close error", func(t *testing.T) {
		wantErr := errors.New("close failed")
		stub := &closeBPFStub{closeErr: wantErr}

		if err := closeBPF(stub); !errors.Is(err, wantErr) {
			t.Fatalf("closeBPF() error = %v, want %v", err, wantErr)
		}
		if !stub.closed {
			t.Fatal("closeBPF() did not call BPF.Close")
		}
	})
}
