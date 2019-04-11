// Copyright 2019 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package scheduler

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sorintlab/agola/internal/services/runservice/types"
)

func TestAdvanceRunTasks(t *testing.T) {
	// a global run config for all tests
	rc := &types.RunConfig{
		Tasks: map[string]*types.RunConfigTask{
			"task01": &types.RunConfigTask{
				ID:      "task01",
				Name:    "task01",
				Depends: []*types.RunConfigTaskDepend{},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task02": &types.RunConfigTask{
				ID:   "task02",
				Name: "task02",
				Depends: []*types.RunConfigTaskDepend{
					&types.RunConfigTaskDepend{
						TaskID: "task01",
					},
				},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task03": &types.RunConfigTask{
				ID:      "task03",
				Name:    "task03",
				Depends: []*types.RunConfigTaskDepend{},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task04": &types.RunConfigTask{
				ID:   "task04",
				Name: "task04",
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task05": &types.RunConfigTask{
				ID:   "task05",
				Name: "task05",
				Depends: []*types.RunConfigTaskDepend{
					&types.RunConfigTaskDepend{TaskID: "task03"},
					&types.RunConfigTaskDepend{TaskID: "task04"},
				},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
		},
	}

	// initial run that matched the runconfig, all tasks are not started or skipped
	// (if the runconfig task as Skip == true). This must match the status
	// generated by command.genRun()
	run := &types.Run{
		RunTasks: map[string]*types.RunTask{
			"task01": &types.RunTask{
				ID:     "task01",
				Status: types.RunTaskStatusNotStarted,
			},
			"task02": &types.RunTask{
				ID:     "task02",
				Status: types.RunTaskStatusNotStarted,
			},
			"task03": &types.RunTask{
				ID:     "task03",
				Status: types.RunTaskStatusNotStarted,
			},
			"task04": &types.RunTask{
				ID:     "task04",
				Status: types.RunTaskStatusNotStarted,
			},
			"task05": &types.RunTask{
				ID:     "task05",
				Status: types.RunTaskStatusNotStarted,
			},
		},
	}

	tests := []struct {
		name string
		rc   *types.RunConfig
		r    *types.Run
		out  *types.Run
		err  error
	}{
		{
			name: "test top level task not started",
			rc:   rc,
			r:    run.DeepCopy(),
			out:  run.DeepCopy(),
		},
		{
			name: "test task status set to skipped when parent status is skipped",
			rc: func() *types.RunConfig {
				rc := rc.DeepCopy()
				rc.Tasks["task01"].Skip = true
				return rc
			}(),
			r: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task01"].Status = types.RunTaskStatusSkipped
				return run
			}(),
			out: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task01"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task02"].Status = types.RunTaskStatusSkipped
				return run
			}(),
		},
		{
			name: "test task status set to skipped when all parent status is skipped",
			rc: func() *types.RunConfig {
				rc := rc.DeepCopy()
				rc.Tasks["task03"].Skip = true
				rc.Tasks["task04"].Skip = true
				return rc
			}(),
			r: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task03"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task04"].Status = types.RunTaskStatusSkipped
				return run
			}(),
			out: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task03"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task04"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task05"].Status = types.RunTaskStatusSkipped
				return run
			}(),
		},
		{
			name: "test task status not set to skipped when not all parent status is skipped",
			rc: func() *types.RunConfig {
				rc := rc.DeepCopy()
				rc.Tasks["task03"].Skip = true
				return rc
			}(),
			r: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task03"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task04"].Status = types.RunTaskStatusSuccess
				return run
			}(),
			out: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task03"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task04"].Status = types.RunTaskStatusSuccess
				return run
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if err := advanceRunTasks(ctx, tt.r, tt.rc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.out, tt.r); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestGetTasksToRun(t *testing.T) {
	// a global run config for all tests
	rc := &types.RunConfig{
		Tasks: map[string]*types.RunConfigTask{
			"task01": &types.RunConfigTask{
				ID:      "task01",
				Name:    "task01",
				Depends: []*types.RunConfigTaskDepend{},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task02": &types.RunConfigTask{
				ID:   "task02",
				Name: "task02",
				Depends: []*types.RunConfigTaskDepend{
					&types.RunConfigTaskDepend{
						TaskID: "task01",
					},
				},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task03": &types.RunConfigTask{
				ID:      "task03",
				Name:    "task03",
				Depends: []*types.RunConfigTaskDepend{},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task04": &types.RunConfigTask{
				ID:   "task04",
				Name: "task04",
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
			"task05": &types.RunConfigTask{
				ID:   "task05",
				Name: "task05",
				Depends: []*types.RunConfigTaskDepend{
					&types.RunConfigTaskDepend{TaskID: "task03"},
					&types.RunConfigTaskDepend{TaskID: "task04"},
				},
				Runtime: &types.Runtime{Type: types.RuntimeType("pod"),
					Containers: []*types.Container{{Image: "image01"}},
				},
				Environment: map[string]string{},
				Steps:       []interface{}{},
				Skip:        false,
			},
		},
	}

	// initial run that matched the runconfig, all tasks are not started or skipped
	// (if the runconfig task as Skip == true). This must match the status
	// generated by command.genRun()
	run := &types.Run{
		RunTasks: map[string]*types.RunTask{
			"task01": &types.RunTask{
				ID:     "task01",
				Status: types.RunTaskStatusNotStarted,
			},
			"task02": &types.RunTask{
				ID:     "task02",
				Status: types.RunTaskStatusNotStarted,
			},
			"task03": &types.RunTask{
				ID:     "task03",
				Status: types.RunTaskStatusNotStarted,
			},
			"task04": &types.RunTask{
				ID:     "task04",
				Status: types.RunTaskStatusNotStarted,
			},
			"task05": &types.RunTask{
				ID:     "task05",
				Status: types.RunTaskStatusNotStarted,
			},
		},
	}

	tests := []struct {
		name string
		rc   *types.RunConfig
		r    *types.Run
		out  []string
		err  error
	}{
		{
			name: "test run top level tasks",
			rc:   rc,
			r:    run.DeepCopy(),
			out:  []string{"task01", "task03", "task04"},
		},
		{
			name: "test don't run skipped tasks",
			rc: func() *types.RunConfig {
				rc := rc.DeepCopy()
				rc.Tasks["task01"].Skip = true
				return rc
			}(),
			r: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task01"].Status = types.RunTaskStatusSkipped
				run.RunTasks["task02"].Status = types.RunTaskStatusSkipped
				return run
			}(),
			out: []string{"task03", "task04"},
		},
		{
			name: "test don't run if needs approval but not approved",
			rc: func() *types.RunConfig {
				rc := rc.DeepCopy()
				rc.Tasks["task01"].NeedsApproval = true
				return rc
			}(),
			r:   run.DeepCopy(),
			out: []string{"task03", "task04"},
		},
		{
			name: "test run if needs approval and approved",
			rc: func() *types.RunConfig {
				rc := rc.DeepCopy()
				rc.Tasks["task01"].NeedsApproval = true
				return rc
			}(),
			r: func() *types.Run {
				run := run.DeepCopy()
				run.RunTasks["task01"].Approved = true
				return run
			}(),
			out: []string{"task01", "task03", "task04"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tasks, err := getTasksToRun(ctx, tt.r, tt.rc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			outTasks := []string{}
			for _, t := range tasks {
				outTasks = append(outTasks, t.ID)
			}
			sort.Sort(sort.StringSlice(tt.out))
			sort.Sort(sort.StringSlice(outTasks))

			if diff := cmp.Diff(tt.out, outTasks); diff != "" {
				t.Error(diff)
			}
		})
	}
}
