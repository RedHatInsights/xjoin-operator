package common

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"time"
)

type Iteration struct {
	Instance         client.Object
	OriginalInstance client.Object
	Client           client.Client
	Context          context.Context
	Log              xjoinlogger.Log
}

func (i *Iteration) UpdateStatusAndRequeue() (reconcile.Result, error) {
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
	}

	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
}

func (i *Iteration) CreateChildResource(resourceDefinition unstructured.Unstructured) (err error) {
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
		APIVersion:         apiVersion.Interface().(string),
		Kind:               kind.Interface().(string),
		Name:               i.Instance.GetName(),
		UID:                i.Instance.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
	resourceDefinition.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Create(ctx, &resourceDefinition)
	return
}

func (i *Iteration) DeleteResource(name string, gvk schema.GroupVersionKind) error {
	dataSourcePipeline := &unstructured.Unstructured{}
	dataSourcePipeline.SetGroupVersionKind(gvk)
	dataSourcePipeline.SetName(name)
	dataSourcePipeline.SetNamespace(i.Instance.GetNamespace())
	return i.Client.Delete(i.Context, dataSourcePipeline)
}

func (i *Iteration) AddFinalizer(finalizer string) error {
	if !utils.ContainsString(i.Instance.GetFinalizers(), finalizer) {
		controllerutil.AddFinalizer(i.Instance, finalizer)

		return i.Client.Update(i.Context, i.Instance)
	}

	return nil
}

func (i *Iteration) Version() string {
	return fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
}
