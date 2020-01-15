/*
Copyright 2018 The Volcano Authors.

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

package validate

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	batchv1alpha1 "volcano.sh/volcano/pkg/apis/batch/v1alpha1"
)

// policyEventMap defines all policy events and whether to allow external use
var policyEventMap = map[batchv1alpha1.Event]bool{
	batchv1alpha1.AnyEvent:           true,
	batchv1alpha1.PodFailedEvent:     true,
	batchv1alpha1.PodEvictedEvent:    true,
	batchv1alpha1.JobUnknownEvent:    true,
	batchv1alpha1.TaskCompletedEvent: true,
	batchv1alpha1.OutOfSyncEvent:     false,
	batchv1alpha1.CommandIssuedEvent: false,
}

// policyActionMap defines all policy actions and whether to allow external use
var policyActionMap = map[batchv1alpha1.Action]bool{
	batchv1alpha1.AbortJobAction:     true,
	batchv1alpha1.RestartJobAction:   true,
	batchv1alpha1.RestartTaskAction:  true,
	batchv1alpha1.TerminateJobAction: true,
	batchv1alpha1.CompleteJobAction:  true,
	batchv1alpha1.ResumeJobAction:    true,
	batchv1alpha1.SyncJobAction:      false,
	batchv1alpha1.EnqueueAction:      false,
}

func validatePolicies(policies []batchv1alpha1.LifecyclePolicy, fldPath *field.Path) error {
	var err error
	policyEvents := map[batchv1alpha1.Event]struct{}{}
	exitCodes := map[int32]struct{}{}

	for _, policy := range policies {
		if (policy.Event != "" || len(policy.Events) != 0) && policy.ExitCode != nil {
			err = multierror.Append(err, fmt.Errorf("must not specify event and exitCode simultaneously"))
			break
		}

		if policy.Event == "" && len(policy.Events) == 0 && policy.ExitCode == nil {
			err = multierror.Append(err, fmt.Errorf("either event and exitCode should be specified"))
			break
		}

		if len(policy.Event) != 0 || len(policy.Events) != 0 {
			bFlag := false
			policyEventsList := getEventList(policy)
			for _, event := range policyEventsList {
				if allow, ok := policyEventMap[event]; !ok || !allow {
					err = multierror.Append(err, field.Invalid(fldPath, event, fmt.Sprintf("invalid policy event")))
					bFlag = true
					break
				}

				if allow, ok := policyActionMap[policy.Action]; !ok || !allow {
					err = multierror.Append(err, field.Invalid(fldPath, policy.Action, fmt.Sprintf("invalid policy action")))
					bFlag = true
					break
				}
				if _, found := policyEvents[event]; found {
					err = multierror.Append(err, fmt.Errorf("duplicate event %v  across different policy", event))
					bFlag = true
					break
				} else {
					policyEvents[event] = struct{}{}
				}
			}
			if bFlag == true {
				break
			}

		} else {
			if *policy.ExitCode == 0 {
				err = multierror.Append(err, fmt.Errorf("0 is not a valid error code"))
				break
			}
			if _, found := exitCodes[*policy.ExitCode]; found {
				err = multierror.Append(err, fmt.Errorf("duplicate exitCode %v", *policy.ExitCode))
				break
			} else {
				exitCodes[*policy.ExitCode] = struct{}{}
			}
		}
	}

	if _, found := policyEvents[batchv1alpha1.AnyEvent]; found && len(policyEvents) > 1 {
		err = multierror.Append(err, fmt.Errorf("if there's * here, no other policy should be here"))
	}

	return err
}

func getEventList(policy batchv1alpha1.LifecyclePolicy) []batchv1alpha1.Event {
	policyEventsList := policy.Events
	if len(policy.Event) > 0 {
		policyEventsList = append(policyEventsList, policy.Event)
	}
	uniquePolicyEventlist := removeDuplicates(policyEventsList)
	return uniquePolicyEventlist
}

func removeDuplicates(EventList []batchv1alpha1.Event) []batchv1alpha1.Event {
	keys := make(map[batchv1alpha1.Event]bool)
	list := []batchv1alpha1.Event{}
	for _, val := range EventList {
		if _, value := keys[val]; !value {
			keys[val] = true
			list = append(list, val)
		}
	}
	return list
}

func getValidEvents() []batchv1alpha1.Event {
	var events []batchv1alpha1.Event
	for e, allow := range policyEventMap {
		if allow {
			events = append(events, e)
		}
	}

	return events
}

func getValidActions() []batchv1alpha1.Action {
	var actions []batchv1alpha1.Action
	for a, allow := range policyActionMap {
		if allow {
			actions = append(actions, a)
		}
	}

	return actions
}

// validateIO validates IO configuration
func validateIO(volumes []batchv1alpha1.VolumeSpec) error {
	volumeMap := map[string]bool{}
	for _, volume := range volumes {
		if len(volume.MountPath) == 0 {
			return fmt.Errorf(" mountPath is required;")
		}
		if _, found := volumeMap[volume.MountPath]; found {
			return fmt.Errorf(" duplicated mountPath: %s;", volume.MountPath)
		}
		if volume.VolumeClaim == nil && volume.VolumeClaimName == "" {
			return fmt.Errorf(" either VolumeClaim or VolumeClaimName must be specified;")
		}
		if len(volume.VolumeClaimName) != 0 {
			if volume.VolumeClaim != nil {
				return fmt.Errorf("confilct: If you want to use an existing PVC, just specify VolumeClaimName." +
					"If you want to create a new PVC, you do not need to specify VolumeClaimName")
			}
			if errMsgs := validation.ValidatePersistentVolumeName(volume.VolumeClaimName, false); len(errMsgs) > 0 {
				return fmt.Errorf("invalid VolumeClaimName %s : %v", volume.VolumeClaimName, errMsgs)
			}
		}

		volumeMap[volume.MountPath] = true
	}
	return nil
}