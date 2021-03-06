// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"cocoon/db"
	"errors"
)

// GetPublicStatusResult contains the anticipated build status.
type GetPublicStatusResult struct {
	AnticipatedBuildStatus db.BuildResult
}

// GetPublicBuildStatus returns latest build status.
func GetPublicBuildStatus(c *db.Cocoon, _ []byte) (interface{}, error) {
	statuses, err := c.QueryBuildStatusesWithMemcache()

	if err != nil {
		return nil, err
	}

	trend := computeTrend(statuses)

	if trend == db.BuildNew {
		return nil, errors.New("No successful or failed builds found. The system might be having trouble catching up with the rate of commits.")
	}

	return &GetPublicStatusResult{
		AnticipatedBuildStatus: trend,
	}, nil
}

// Computes anticipated outcome given the build status. If the latest task statuses are all
// successful, anticipates build success. Otherwise anticipates failure. If there are no finished
// tasks at all, returns [BuildNew].
func computeTrend(statuses []*db.BuildStatus) db.BuildResult {
	checkedTasks := map[string]bool{}

	isLatestBuild := true
	for _, status := range statuses {
		for _, stage := range status.Stages {
			for _, task := range stage.Tasks {
				if isLatestBuild {
					// We only care about tasks defined in the latest build. If a task is removed from CI, we
					// no longer care about its status.
					checkedTasks[task.Task.Name] = false
				}

				if checked, isInLatestBuild := checkedTasks[task.Task.Name]; isInLatestBuild && !checked && (task.Task.Flaky || task.Task.Status.IsFinal()) {
					checkedTasks[task.Task.Name] = true
					if !task.Task.Flaky && (task.Task.Status == db.TaskFailed || task.Task.Status == db.TaskSkipped) {
						return db.BuildWillFail
					}
				}
			}
		}
		isLatestBuild = false
	}

	if len(checkedTasks) == 0 {
		return db.BuildWillFail
	}

	return db.BuildSucceeded
}
