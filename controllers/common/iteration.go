package common

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"github.com/redhatinsights/xjoin-operator/controllers/common/labels"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type Iteration struct {
	Instance         XJoinObjectChild
	OriginalInstance client.Object
	Client           client.Client
	Context          context.Context
	Log              xjoinlogger.Log
	Test             bool
}

func UpdateCondition(instance XJoinObjectChild) {
	if instance.GetValidationResult() == validation.ValidationValid {
		instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionTrue,
			Reason: ValidationSucceededReason,
		})
	} else if instance.GetValidationResult() == validation.ValidationInvalid {
		instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: ValidationFailedReason,
		})
	} else if instance.GetValidationResult() == validation.ValidationNew {
		instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: NewReason,
		})
	} else {
		instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: UnknownReason,
		})
	}
}

func (i *Iteration) UpdateCondition() {
	UpdateCondition(i.Instance)
}

func (i *Iteration) UpdateStatusAndRequeue(requeueAfter time.Duration) (reconcile.Result, error) {
	instanceVal := reflect.ValueOf(i.Instance).Elem()
	statusVal := instanceVal.FieldByName("Status")
	if !statusVal.IsValid() {
		err := errors.New("status field not found on instance")
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	originalInstanceVal := reflect.ValueOf(i.OriginalInstance).Elem()
	originalStatusVal := originalInstanceVal.FieldByName("Status")
	if !originalStatusVal.IsValid() {
		err := errors.New("status field not found on original instance")
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	// Only issue status update if Reconcile actually modified Status
	// This prevents write conflicts between the controllers
	if !cmp.Equal(statusVal.Interface(), originalStatusVal.Interface()) {
		i.Log.Debug("Updating status")

		ctx, cancel := utils.DefaultContext()
		defer cancel()

		if err := i.Client.Status().Update(ctx, i.Instance); err != nil {
			if k8errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, err
		}

		time.Sleep(1 * time.Second) //TODO: this is to prevent status conflicts. Find a better way to avoid the conflicts
	}

	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (i *Iteration) CreateChildResource(resourceDefinition client.Object, ownerGVK schema.GroupVersionKind) (err error) {
	instanceVal := reflect.ValueOf(i.Instance).Elem()
	apiVersion := instanceVal.FieldByName("APIVersion")
	if !apiVersion.IsValid() {
		err := errors.New("status field not found on original instance")
		return errors.Wrap(err, 0)
	}

	kind := instanceVal.FieldByName("Kind")
	if !kind.IsValid() {
		err := errors.New("status field not found on original instance")
		return errors.Wrap(err, 0)
	}

	blockOwnerDeletion := true
	controller := true
	ownerReference := metav1.OwnerReference{
		APIVersion:         ownerGVK.Group + "/" + ownerGVK.Version,
		Kind:               ownerGVK.Kind,
		Name:               i.Instance.GetName(),
		UID:                i.Instance.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
	resourceDefinition.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Create(ctx, resourceDefinition)
	return
}

func (i *Iteration) DeleteResource(name string, gvk schema.GroupVersionKind) error {
	//check if resources exists before trying to delete it
	listResource := &unstructured.UnstructuredList{}
	listResource.SetGroupVersionKind(gvk)
	err := i.Client.List(i.Context, listResource)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	//delete resource if it exists
	for _, item := range listResource.Items {
		if item.GetName() == name {
			resource := &unstructured.Unstructured{}
			resource.SetGroupVersionKind(gvk)
			resource.SetName(name)
			resource.SetNamespace(i.Instance.GetNamespace())

			err = i.Client.Delete(i.Context, resource)
			if err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}

	return nil
}

func (i *Iteration) AddFinalizer(finalizer string) error {
	if !utils.ContainsString(i.Instance.GetFinalizers(), finalizer) {
		controllerutil.AddFinalizer(i.Instance, finalizer)

		return i.Client.Update(i.Context, i.Instance)
	}

	return nil
}

func (i *Iteration) ReconcileChild(child Child, labelsMatch client.MatchingLabels) (err error) {
	//build an array and map of expected child versions (active, refreshing)
	//the map value will be set to true when an expected child is found
	expectedChildrenMap := make(map[string]bool)
	var expectedChildrenArray []string
	if child.GetParentInstance().GetActiveVersion() != "" {
		expectedChildrenMap[child.GetParentInstance().GetActiveVersion()] = false
		expectedChildrenArray = append(expectedChildrenArray, child.GetParentInstance().GetActiveVersion())
	}
	if child.GetParentInstance().GetRefreshingVersion() != "" {
		expectedChildrenMap[child.GetParentInstance().GetRefreshingVersion()] = false
		expectedChildrenArray = append(expectedChildrenArray, child.GetParentInstance().GetRefreshingVersion())
	}

	//retrieve a list of children for this datasource.name
	children := &unstructured.UnstructuredList{}
	children.SetGroupVersionKind(child.GetGVK())
	err = i.Client.List(i.Context, children, labelsMatch, client.InNamespace(child.GetParentInstance().GetNamespace()))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	apiVersion, kind := child.GetGVK().ToAPIVersionAndKind()

	//remove any extra children, ensure the expected children are created
	for _, childItem := range children.Items {
		spec := childItem.Object["spec"]
		if spec == nil {
			err = errors.New(
				fmt.Sprintf("spec not found in child custom resource. Name: %s, apiVersoin: %s, kind: %s",
					child.GetParentInstance().GetName(), apiVersion, kind))
			return errors.Wrap(err, 0)
		}

		specMap := spec.(map[string]interface{})
		version := specMap["version"]
		if version == nil {
			err = errors.New(
				fmt.Sprintf("version not found in child custom resource's spec. Name: %s, apiVersoin: %s, kind: %s",
					child.GetParentInstance().GetName(), apiVersion, kind))
			return errors.Wrap(err, 0)
		}

		versionString := version.(string)

		if !utils.ContainsString(expectedChildrenArray, versionString) {
			err = child.Delete(versionString)
			if err != nil {
				return errors.Wrap(err, 0)
			}
		} else {
			expectedChildrenMap[versionString] = true
		}
	}

	for version, exists := range expectedChildrenMap {
		if !exists {
			i.Log.Info("expected version "+version+" not found, recreating it.",
				"kind", kind)
			err = child.Create(version)
			if err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}

	return
}

func (i *Iteration) DeleteAllGVKsWithLabels(gvk schema.GroupVersionKind, matchingLabels client.MatchingLabels) (err error) {
	resources := &unstructured.UnstructuredList{}
	resources.SetGroupVersionKind(gvk)

	err = i.Client.List(i.Context, resources, matchingLabels, client.InNamespace(i.Instance.GetNamespace()))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for _, item := range resources.Items {
		resource := &unstructured.Unstructured{}
		resource.SetGroupVersionKind(gvk)
		resource.SetNamespace(i.Instance.GetNamespace())
		resource.SetName(item.GetName())
		err = i.Client.Delete(i.Context, resource)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}

func (i *Iteration) DeleteAllResourceTypeWithComponentName(gvk schema.GroupVersionKind, componentName string) (err error) {
	resources := &unstructured.UnstructuredList{}
	resources.SetGroupVersionKind(gvk)
	labelsMatch := client.MatchingLabels{labels.ComponentName: componentName}

	err = i.Client.List(i.Context, resources, labelsMatch, client.InNamespace(i.Instance.GetNamespace()))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for _, item := range resources.Items {
		resource := &unstructured.Unstructured{}
		resource.SetGroupVersionKind(gvk)
		resource.SetNamespace(i.Instance.GetNamespace())
		resource.SetName(item.GetName())
		err = i.Client.Delete(i.Context, resource)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}
