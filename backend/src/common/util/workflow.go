// Copyright 2018 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"strings"

	workflowapi "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	swfregister "github.com/kubeflow/pipelines/backend/src/crd/pkg/apis/scheduledworkflow"
	swfapi "github.com/kubeflow/pipelines/backend/src/crd/pkg/apis/scheduledworkflow/v1beta1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
)

// Workflow is a type to help manipulate Workflow objects.
type Workflow struct {
	*workflowapi.Workflow
}

func NewWorkflowFromBytes(bytes []byte) (*Workflow, error) {
	var workflow workflowapi.Workflow
	err := yaml.Unmarshal(bytes, &workflow)
	if err != nil {
		return nil, NewInvalidInputErrorWithDetails(err, "Failed to unmarshal the inputs")
	}
	return NewWorkflow(&workflow), nil
}

func NewWorkflowFromBytesJSON(bytes []byte) (*Workflow, error) {
	var workflow workflowapi.Workflow
	err := json.Unmarshal(bytes, &workflow)
	if err != nil {
		return nil, NewInvalidInputErrorWithDetails(err, "Failed to unmarshal the inputs")
	}
	return NewWorkflow(&workflow), nil
}

func NewWorkflowFromInterface(obj interface{}) (*Workflow, error) {
	workflow, ok := obj.(*workflowapi.Workflow)
	if ok {
		return NewWorkflow(workflow), nil
	}
	return nil, NewInvalidInputError("not Workflow struct")
}

// NewWorkflow creates a Workflow.
func NewWorkflow(workflow *workflowapi.Workflow) *Workflow {
	return &Workflow{
		workflow,
	}
}

func UnmarshParametersWorkflow(paramsString string) (SpecParameters, error) {
	if paramsString == "" {
		return nil, nil
	}
	var params []workflowapi.Parameter
	err := json.Unmarshal([]byte(paramsString), &params)
	if err != nil {
		return nil, NewInternalServerError(err, "Parameters have wrong format")
	}
	rev := make(SpecParameters, 0, len(params))
	for _, param := range params {
		rev = append(rev, SpecParameter{
			Name:    param.Name,
			Default: (*string)(param.Default),
			Value:   (*string)(param.Value)})
	}
	return rev, nil
}

// Marshal parameters to JSON encoded string.
// This also checks result is not longer than a limit.
func MarshalParametersWorkflow(params SpecParameters) (string, error) {
	if params == nil {
		return "[]", nil
	}

	inputParams := make([]workflowapi.Parameter, 0)
	for _, param := range params {
		newParam := workflowapi.Parameter{
			Name:    param.Name,
			Default: ToAnyStringPointer(param.Default),
			Value:   ToAnyStringPointer(param.Value),
		}
		inputParams = append(inputParams, newParam)
	}
	paramBytes, err := json.Marshal(inputParams)
	if err != nil {
		return "", NewInvalidInputErrorWithDetails(err, "Failed to marshal the parameter.")
	}
	if len(paramBytes) > MaxParameterBytes {
		return "", NewInvalidInputError("The input parameter length exceed maximum size of %v.", MaxParameterBytes)
	}
	return string(paramBytes), nil
}

// Get ExecutionType: ArgoWorkflow
func (w *Workflow) ExecutionType() ExecutionType {
	return ArgoWorkflow
}

// ExecutionSpec interface: Get ExecutionStatus which can be used to
// access status related information
func (w *Workflow) ExecutionStatus() ExecutionStatus {
	return w
}

// SetServiceAccount Set the service account to run the workflow.
func (w *Workflow) SetServiceAccount(serviceAccount string) {
	w.Spec.ServiceAccountName = serviceAccount
}

func (w *Workflow) ServiceAccount() string {
	return w.Spec.ServiceAccountName
}

func (w *Workflow) SpecParameters() SpecParameters {
	rev := make(SpecParameters, 0, len(w.Spec.Arguments.Parameters))
	for _, currentParam := range w.Spec.Arguments.Parameters {
		rev = append(rev, SpecParameter{
			Name:    currentParam.Name,
			Default: (*string)(currentParam.Default),
			Value:   (*string)(currentParam.Value)})
	}
	return rev
}

func (w *Workflow) SetSpecParameters(params SpecParameters) {
	desiredSlice := make([]workflowapi.Parameter, 0)
	for _, currentParam := range params {
		newParam := workflowapi.Parameter{
			Name:    currentParam.Name,
			Default: ToAnyStringPointer(currentParam.Default),
			Value:   ToAnyStringPointer(currentParam.Value),
		}
		desiredSlice = append(desiredSlice, newParam)
	}
	w.Spec.Arguments.Parameters = desiredSlice
}

func (w *Workflow) GenerateRetryExecution() (ExecutionSpec, []string, error) {
	switch w.Status.Phase {
	case workflowapi.WorkflowFailed, workflowapi.WorkflowError:
		break
	default:
		return nil, nil, NewBadRequestError(errors.New("workflow cannot be retried"), "Workflow must be Failed/Error to retry")
	}

	newWF := w.Workflow.DeepCopy()
	// Delete/reset fields which indicate workflow completed
	delete(newWF.Labels, common.LabelKeyCompleted)
	// Delete/reset fields which indicate workflow is finished being persisted to the database
	delete(newWF.Labels, LabelKeyWorkflowPersistedFinalState)
	newWF.ObjectMeta.Labels[common.LabelKeyPhase] = string(workflowapi.NodeRunning)
	newWF.Status.Phase = workflowapi.WorkflowRunning
	newWF.Status.Message = ""
	newWF.Status.FinishedAt = metav1.Time{}
	if newWF.Spec.ActiveDeadlineSeconds != nil && *newWF.Spec.ActiveDeadlineSeconds == 0 {
		// if it was terminated, unset the deadline
		newWF.Spec.ActiveDeadlineSeconds = nil
	}

	// Iterate the previous nodes. If it was successful Pod carry it forward
	newWF.Status.Nodes = make(map[string]workflowapi.NodeStatus)
	onExitNodeName := w.ObjectMeta.Name + ".onExit"
	var podsToDelete []string
	for _, node := range w.Status.Nodes {
		switch node.Phase {
		case workflowapi.NodeSucceeded, workflowapi.NodeSkipped:
			if !strings.HasPrefix(node.Name, onExitNodeName) {
				newWF.Status.Nodes[node.ID] = node
				continue
			}
		case workflowapi.NodeError, workflowapi.NodeFailed, workflowapi.NodeOmitted:
			if !strings.HasPrefix(node.Name, onExitNodeName) && node.Type == workflowapi.NodeTypeDAG {
				newNode := node.DeepCopy()
				newNode.Phase = workflowapi.NodeRunning
				newNode.Message = ""
				newNode.FinishedAt = metav1.Time{}
				newWF.Status.Nodes[newNode.ID] = *newNode
				continue
			}
			// do not add this status to the node. pretend as if this node never existed.
		default:
			// Do not allow retry of workflows with pods in Running/Pending phase
			return nil, nil, NewInternalServerError(
				errors.New("workflow cannot be retried"),
				"Workflow cannot be retried with node %s in %s phase", node.ID, node.Phase)
		}
		if node.Type == workflowapi.NodeTypePod {
			podsToDelete = append(podsToDelete, node.ID)
		}
	}
	return NewWorkflow(newWF), podsToDelete, nil
}

func (w *Workflow) Version() string {
	return w.ResourceVersion
}

func (w *Workflow) SetVersion(version string) {
	w.ResourceVersion = version
}

func (w *Workflow) ExecutionName() string {
	return w.Name
}

func (w *Workflow) SetExecutionName(name string) {
	w.Name = name
}

func (w *Workflow) ExecutionNamespace() string {
	return w.Namespace
}

func (w *Workflow) SetExecutionNamespace(namespace string) {
	w.Namespace = namespace
}

func (w *Workflow) ExecutionUID() string {
	return string(w.UID)
}

func (w *Workflow) ExecutionMeta() metav1.ObjectMeta {
	return w.ObjectMeta
}

func (w *Workflow) IsTerminating() bool {
	return w.Spec.ActiveDeadlineSeconds != nil &&
		*w.Spec.ActiveDeadlineSeconds == 0 &&
		!w.IsInFinalState()
}

// OverrideParameters overrides some of the parameters of a Workflow.
func (w *Workflow) OverrideParameters(desiredParams map[string]string) {
	desiredSlice := make([]workflowapi.Parameter, 0)
	for _, currentParam := range w.Spec.Arguments.Parameters {
		var desiredValue *string = nil
		if param, ok := desiredParams[currentParam.Name]; ok {
			desiredValue = &param
		} else if currentParam.Value != nil {
			desired := currentParam.Value.String()
			desiredValue = &desired
		}

		desiredSlice = append(desiredSlice, workflowapi.Parameter{
			Name:  currentParam.Name,
			Value: ToAnyStringPointer(desiredValue),
		})
	}
	w.Spec.Arguments.Parameters = desiredSlice
}

func (w *Workflow) GetWorkflowParametersAsMap() map[string]string {
	resultAsArray := w.Spec.Arguments.Parameters
	resultAsMap := make(map[string]string)
	for _, param := range resultAsArray {
		if param.Value == nil {
			resultAsMap[param.Name] = ""
		} else {
			resultAsMap[param.Name] = param.Value.String()
		}
	}
	return resultAsMap
}

func (w *Workflow) VerifyParameters(desiredParams map[string]string) error {
	templateParamsMap := make(map[string]*string)
	for _, param := range w.Spec.Arguments.Parameters {
		templateParamsMap[param.Name] = ToStringPointer(param.Value)
	}
	for k := range desiredParams {
		_, ok := templateParamsMap[k]
		if !ok {
			return NewInvalidInputError("Unrecognized input parameter: %v", k)
		}
	}
	return nil
}

// Get converts this object to a workflowapi.Workflow.
func (w *Workflow) Get() *workflowapi.Workflow {
	return w.Workflow
}

func (w *Workflow) ScheduledWorkflowUUIDAsStringOrEmpty() string {
	if w.OwnerReferences == nil {
		return ""
	}

	for _, reference := range w.OwnerReferences {
		if isScheduledWorkflow(reference) {
			return string(reference.UID)
		}
	}

	return ""
}

func containsScheduledWorkflow(references []metav1.OwnerReference) bool {
	if references == nil {
		return false
	}

	for _, reference := range references {
		if isScheduledWorkflow(reference) {
			return true
		}
	}

	return false
}

func isScheduledWorkflow(reference metav1.OwnerReference) bool {
	gvk := schema.GroupVersionKind{
		Group:   swfapi.SchemeGroupVersion.Group,
		Version: swfapi.SchemeGroupVersion.Version,
		Kind:    swfregister.Kind,
	}

	if reference.APIVersion == gvk.GroupVersion().String() &&
		reference.Kind == gvk.Kind &&
		reference.UID != "" {
		return true
	}
	return false
}

func (w *Workflow) ScheduledAtInSecOr0() int64 {
	if w.Labels == nil {
		return 0
	}

	for key, value := range w.Labels {
		if key == LabelKeyWorkflowEpoch {
			result, err := RetrieveInt64FromLabel(value)
			if err != nil {
				glog.Errorf("Could not retrieve scheduled epoch from label key (%v) and label value (%v).", key, value)
				return 0
			}
			return result
		}
	}

	return 0
}

func (w *Workflow) FinishedAt() int64 {
	if w.Status.FinishedAt.IsZero() {
		// If workflow is not finished
		return 0
	}
	return w.Status.FinishedAt.Unix()
}

func (w *Workflow) Condition() ExecutionPhase {
	return ExecutionPhase(w.Status.Phase)
}

func (w *Workflow) ToStringForStore() string {
	workflow, err := json.Marshal(w.Workflow)
	if err != nil {
		glog.Errorf("Could not marshal the workflow: %v", w.Workflow)
		return ""
	}
	return string(workflow)
}

func (w *Workflow) HasScheduledWorkflowAsParent() bool {
	return containsScheduledWorkflow(w.Workflow.OwnerReferences)
}

func (w *Workflow) GetExecutionSpec() ExecutionSpec {
	workflow := w.DeepCopy()
	workflow.Status = workflowapi.WorkflowStatus{}
	workflow.TypeMeta = metav1.TypeMeta{Kind: w.Kind, APIVersion: w.APIVersion}
	// To prevent collisions, clear name, set GenerateName to first 200 runes of previous name.
	nameRunes := []rune(w.Name)
	length := len(nameRunes)
	if length > 200 {
		length = 200
	}
	workflow.ObjectMeta = metav1.ObjectMeta{GenerateName: string(nameRunes[:length])}
	return NewWorkflow(workflow)
}

// OverrideName sets the name of a Workflow.
func (w *Workflow) OverrideName(name string) {
	w.GenerateName = ""
	w.Name = name
}

// SetAnnotationsToAllTemplatesIfKeyNotExist sets annotations on all templates in a Workflow
// if the annotation key does not exist
func (w *Workflow) SetAnnotationsToAllTemplatesIfKeyNotExist(key string, value string) {
	if len(w.Spec.Templates) == 0 {
		return
	}
	for index := range w.Spec.Templates {
		if w.Spec.Templates[index].Metadata.Annotations == nil {
			w.Spec.Templates[index].Metadata.Annotations = make(map[string]string)
		}
		if _, isSet := w.Spec.Templates[index].Metadata.Annotations[key]; isSet {
			continue
		}
		w.Spec.Templates[index].Metadata.Annotations[key] = value
	}
}

// SetLabels sets labels on all templates in a Workflow
func (w *Workflow) SetLabelsToAllTemplates(key string, value string) {
	if len(w.Spec.Templates) == 0 {
		return
	}
	for index := range w.Spec.Templates {
		if w.Spec.Templates[index].Metadata.Labels == nil {
			w.Spec.Templates[index].Metadata.Labels = make(map[string]string)
		}
		if w.Spec.Templates[index].Metadata.Labels[key] != value {
			w.Spec.Templates[index].Metadata.Labels[key] = value
		}
	}
}

// SetOwnerReferences sets owner references on a Workflow.
func (w *Workflow) SetOwnerReferences(schedule *swfapi.ScheduledWorkflow) {
	w.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(schedule, schema.GroupVersionKind{
			Group:   swfapi.SchemeGroupVersion.Group,
			Version: swfapi.SchemeGroupVersion.Version,
			Kind:    swfregister.Kind,
		}),
	}
}

func (w *Workflow) SetLabels(key string, value string) {
	if w.Labels == nil {
		w.Labels = make(map[string]string)
	}
	w.Labels[key] = value
}

func (w *Workflow) SetAnnotations(key string, value string) {
	if w.Annotations == nil {
		w.Annotations = make(map[string]string)
	}
	w.Annotations[key] = value
}

func (w *Workflow) SetPodMetadataLabels(key string, value string) {
	if w.Workflow.Spec.PodMetadata == nil {
		w.Workflow.Spec.PodMetadata = &workflowapi.Metadata{}
	}
	if w.Workflow.Spec.PodMetadata.Labels == nil {
		w.Workflow.Spec.PodMetadata.Labels = make(map[string]string)
	}
	w.Workflow.Spec.PodMetadata.Labels[key] = value
}

func (w *Workflow) ReplaceUID(id string) error {
	newWorkflowString := strings.Replace(w.ToStringForStore(), "{{workflow.uid}}", id, -1)
	var workflow *workflowapi.Workflow
	if err := json.Unmarshal([]byte(newWorkflowString), &workflow); err != nil {
		return NewInternalServerError(err,
			"Failed to unmarshal workflow spec manifest. Workflow: %s", w.ToStringForStore())
	}
	w.Workflow = workflow
	return nil
}

func (w *Workflow) SetCannonicalLabels(name string, nextScheduledEpoch int64, index int64) {
	w.SetLabels(LabelKeyWorkflowScheduledWorkflowName, name)
	w.SetLabels(LabelKeyWorkflowEpoch, FormatInt64ForLabel(nextScheduledEpoch))
	w.SetLabels(LabelKeyWorkflowIndex, FormatInt64ForLabel(index))
	w.SetLabels(LabelKeyWorkflowIsOwnedByScheduledWorkflow, "true")
}

// FindObjectStoreArtifactKeyOrEmpty loops through all node running statuses and look up the first
// S3 artifact with the specified nodeID and artifactName. Returns empty if nothing is found.
func (w *Workflow) FindObjectStoreArtifactKeyOrEmpty(nodeID string, artifactName string) string {
	if w.Status.Nodes == nil {
		return ""
	}
	node, found := w.Status.Nodes[nodeID]
	if !found {
		return ""
	}
	if node.Outputs == nil || node.Outputs.Artifacts == nil {
		return ""
	}
	var s3Key string
	for _, artifact := range node.Outputs.Artifacts {
		if artifact.Name != artifactName || artifact.S3 == nil || artifact.S3.Key == "" {
			continue
		}
		s3Key = artifact.S3.Key
	}
	return s3Key
}

// IsInFinalState whether the workflow is in a final state.
func (w *Workflow) IsInFinalState() bool {
	// Workflows in the statuses other than pending or running are considered final.
	if w.Status.Phase == workflowapi.WorkflowSucceeded || w.Status.Phase == workflowapi.WorkflowFailed || w.Status.Phase == workflowapi.WorkflowError {
		return true
	}
	return false
}

// PersistedFinalState whether the workflow final state has being persisted.
func (w *Workflow) PersistedFinalState() bool {
	if _, ok := w.GetLabels()[LabelKeyWorkflowPersistedFinalState]; ok {
		// If the label exist, workflow final state has being persisted.
		return true
	}
	return false
}

// IsV2Compatible whether the workflow is a v2 compatible pipeline.
func (w *Workflow) IsV2Compatible() bool {
	value := w.GetObjectMeta().GetAnnotations()["pipelines.kubeflow.org/v2_pipeline"]
	return value == "true"
}
