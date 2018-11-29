/*
Copyright 2018 The Kubernetes Authors.

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

package util

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	nonExist = "NonExist"
)

// PodsStartupStatus represents status of a pods group.
type PodsStartupStatus struct {
	Expected           int
	Terminating        int
	Running            int
	Scheduled          int
	RunningButNotReady int
	Waiting            int
	Pending            int
	Unknown            int
	Inactive           int
	Created            int
}

// String returns string representation for podsStartupStatus.
func (s *PodsStartupStatus) String() string {
	return fmt.Sprintf("Pods: %d out of %d created, %d running, %d pending scheduled, %d not scheduled, %d inactive, %d terminating, %d unknown, %d runningButNotReady ",
		s.Created, s.Expected, s.Running, s.Pending, s.Waiting, s.Inactive, s.Terminating, s.Unknown, s.RunningButNotReady)
}

// ComputePodsStartupStatus computes PodsStartupStatus for a group of pods.
func ComputePodsStartupStatus(pods []*corev1.Pod, expected int) PodsStartupStatus {
	startupStatus := PodsStartupStatus{
		Expected: expected,
	}
	for _, p := range pods {
		if p.DeletionTimestamp != nil {
			startupStatus.Terminating++
			continue
		}
		startupStatus.Created++
		if p.Status.Phase == corev1.PodRunning {
			ready := false
			for _, c := range p.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					ready = true
					break
				}
			}
			if ready {
				// Only count a pod is running when it is also ready.
				startupStatus.Running++
			} else {
				startupStatus.RunningButNotReady++
			}
		} else if p.Status.Phase == corev1.PodPending {
			if p.Spec.NodeName == "" {
				startupStatus.Waiting++
			} else {
				startupStatus.Pending++
			}
		} else if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			startupStatus.Inactive++
		} else if p.Status.Phase == corev1.PodUnknown {
			startupStatus.Unknown++
		}
		if p.Spec.NodeName != "" {
			startupStatus.Scheduled++
		}
	}
	return startupStatus
}

type podInfo struct {
	oldHostname string
	oldPhase    string
	hostname    string
	phase       string
}

// PodDiff represets diff between old and new group of pods.
type PodDiff map[string]*podInfo

// Print formats and prints the give PodDiff.
func (p PodDiff) String(ignorePhases sets.String) string {
	ret := ""
	for name, info := range p {
		if ignorePhases.Has(info.phase) {
			continue
		}
		if info.phase == nonExist {
			ret += fmt.Sprintf("Pod %v was deleted, had phase %v and host %v\n", name, info.oldPhase, info.oldHostname)
			continue
		}
		phaseChange, hostChange := false, false
		msg := fmt.Sprintf("Pod %v ", name)
		if info.oldPhase != info.phase {
			phaseChange = true
			if info.oldPhase == nonExist {
				msg += fmt.Sprintf("in phase %v ", info.phase)
			} else {
				msg += fmt.Sprintf("went from phase: %v -> %v ", info.oldPhase, info.phase)
			}
		}
		if info.oldHostname != info.hostname {
			hostChange = true
			if info.oldHostname == nonExist || info.oldHostname == "" {
				msg += fmt.Sprintf("assigned host %v ", info.hostname)
			} else {
				msg += fmt.Sprintf("went from host: %v -> %v ", info.oldHostname, info.hostname)
			}
		}
		if phaseChange || hostChange {
			ret += msg + "\n"
		}
	}
	return ret
}

// DeletedPods returns a slice of pods that were present at the beginning
// and then disappeared.
func (p PodDiff) DeletedPods() []string {
	var deletedPods []string
	for podName, podInfo := range p {
		if podInfo.hostname == nonExist {
			deletedPods = append(deletedPods, podName)
		}
	}
	return deletedPods
}

// AddedPods returns a slice of pods that were added.
func (p PodDiff) AddedPods() []string {
	var addedPods []string
	for podName, podInfo := range p {
		if podInfo.oldHostname == nonExist {
			addedPods = append(addedPods, podName)
		}
	}
	return addedPods
}

// DiffPods computes a PodDiff given 2 lists of pods.
func DiffPods(oldPods []*corev1.Pod, curPods []*corev1.Pod) PodDiff {
	podInfoMap := PodDiff{}

	// New pods will show up in the curPods list but not in oldPods. They have oldhostname/phase == nonexist.
	for _, pod := range curPods {
		podInfoMap[pod.Name] = &podInfo{hostname: pod.Spec.NodeName, phase: string(pod.Status.Phase), oldHostname: nonExist, oldPhase: nonExist}
	}

	// Deleted pods will show up in the oldPods list but not in curPods. They have a hostname/phase == nonexist.
	for _, pod := range oldPods {
		if info, ok := podInfoMap[pod.Name]; ok {
			info.oldHostname, info.oldPhase = pod.Spec.NodeName, string(pod.Status.Phase)
		} else {
			podInfoMap[pod.Name] = &podInfo{hostname: nonExist, phase: nonExist, oldHostname: pod.Spec.NodeName, oldPhase: string(pod.Status.Phase)}
		}
	}
	return podInfoMap
}
