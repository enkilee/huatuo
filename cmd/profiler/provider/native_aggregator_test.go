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
	"runtime"
	"slices"
	"testing"

	"huatuo-bamai/internal/profiler"
	pcontext "huatuo-bamai/internal/profiler/context"
	"huatuo-bamai/pkg/profiling"
)

func TestNativeAggregatorAggregatesLockTime(t *testing.T) {
	aggregator := &nativeAggregator{
		aggrMap:     map[string]*stackSample{},
		lockAggrMap: map[string]*lockSample{},
	}
	record := &lockSample{
		Process:         processKey{PID: 12, Comm: "app"},
		LockAddress:     0xab,
		StackTrace:      symbolizedStackTrace{UserFrames: []string{"foo", "bar"}},
		WaitNanoseconds: 10,
		ContentionCount: 2,
	}

	aggregator.Aggregate(record)
	aggregator.Aggregate(record)

	requireSingleLockRecord(t, aggregator, 20, 4)
	_, sampleType, err := profileTypeOptions(&pcontext.ProfilerContext{Type: profiling.TypeLock})
	if err != nil {
		t.Fatalf("profileTypeOptions() error = %v", err)
	}
	if sampleType != profiler.ProfileTypeLockTimeSample {
		t.Fatalf("sample type = %q, want %q", sampleType, profiler.ProfileTypeLockTimeSample)
	}
}

func TestSymbolizedStackTraceAppendToPreservesFrameNames(t *testing.T) {
	trace := symbolizedStackTrace{
		UserFrames:   []string{"generic::<[u8; 7]>", "main"},
		KernelFrames: []string{"entry_SYSCALL_64_after_hwframe"},
	}
	want := []string{
		"process 123:worker",
		"generic::<[u8; 7]>",
		"main",
		"entry_SYSCALL_64_after_hwframe",
	}

	got := trace.appendTo([]string{"process 123:worker"})
	if !slices.Equal(got, want) {
		t.Fatalf("appendTo() = %q, want %q", got, want)
	}
}

func TestNativeAggregatorPreservesStructuredFrameBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		left  symbolizedStackTrace
		right symbolizedStackTrace
	}{
		{
			name:  "literal semicolon is not a frame separator",
			left:  symbolizedStackTrace{UserFrames: []string{"foo;bar"}},
			right: symbolizedStackTrace{UserFrames: []string{"foo", "bar"}},
		},
		{
			name:  "user and kernel sections remain distinct",
			left:  symbolizedStackTrace{UserFrames: []string{"same"}},
			right: symbolizedStackTrace{KernelFrames: []string{"same"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := &nativeAggregator{aggrMap: make(map[string]*stackSample)}
			left := &stackSample{Process: processKey{PID: 12, Comm: "app"}, StackTrace: tt.left}
			right := &stackSample{Process: processKey{PID: 12, Comm: "app"}, StackTrace: tt.right}
			aggregator.Aggregate(left)
			aggregator.Aggregate(right)
			if len(aggregator.aggrMap) != 2 {
				t.Fatalf("aggregated records = %d, want 2", len(aggregator.aggrMap))
			}
		})
	}
}

func TestBuildTreeItemPreservesRawFrameNames(t *testing.T) {
	trace := symbolizedStackTrace{
		UserFrames:   []string{"generic::<[u8; 7]>"},
		KernelFrames: []string{"kernel;frame"},
	}

	item := buildTreeItem([]string{"process 12:app"}, trace, 7)
	want := []string{"process 12:app", "generic::<[u8; 7]>", "kernel;frame"}
	if len(item.Stack) != len(want) {
		t.Fatalf("stack length = %d, want %d", len(item.Stack), len(want))
	}
	for index, frame := range item.Stack {
		if string(frame) != want[index] {
			t.Fatalf("stack[%d] = %q, want %q", index, frame, want[index])
		}
	}
	if item.Value != 7 {
		t.Fatalf("value = %d, want 7", item.Value)
	}
}

func TestUserStackCacheKeyIncludesPID(t *testing.T) {
	first := userStackCacheKey{PID: 100, StackID: 7}
	second := userStackCacheKey{PID: 200, StackID: 7}
	if first == second {
		t.Fatal("user stack cache key aliases the same stack ID across processes")
	}
}

func BenchmarkAppendStackFrames(b *testing.B) {
	prefixes := []string{"process 123:worker", "off-CPU blocked"}
	trace := symbolizedStackTrace{
		UserFrames:   []string{"runtime.goexit", "net/http.(*conn).serve", "main.handle"},
		KernelFrames: []string{"entry_SYSCALL_64_after_hwframe", "do_syscall_64"},
	}

	b.ReportAllocs()
	for b.Loop() {
		frames := trace.appendTo(prefixes[:len(prefixes):len(prefixes)])
		runtime.KeepAlive(frames)
	}
}

func requireSingleLockRecord(t *testing.T, aggregator *nativeAggregator, waitTime uint64, contended uint32) {
	t.Helper()
	if len(aggregator.lockAggrMap) != 1 {
		t.Fatalf("lock records = %d, want 1", len(aggregator.lockAggrMap))
	}
	for _, record := range aggregator.lockAggrMap {
		if record.WaitNanoseconds != waitTime || record.ContentionCount != contended {
			t.Fatalf(
				"lock record = (wait=%d, contended=%d), want (wait=%d, contended=%d)",
				record.WaitNanoseconds,
				record.ContentionCount,
				waitTime,
				contended,
			)
		}
	}
}
