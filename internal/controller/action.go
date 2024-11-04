package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action interface {
	Execute(ctx context.Context) error
}

type PatchStatus struct {
	client   client.Client
	original runtime.Object
	new      runtime.Object
}

func (o *PatchStatus) Execute(ctx context.Context) error {
	if reflect.DeepEqual(o.original, o.new) {
		return nil
	}
	if err := o.client.Status().Patch(ctx, o.new.(client.Object), client.MergeFrom(o.original.(client.Object))); err != nil {
		return fmt.Errorf("while patching status error %q", err)
	}
	return nil
}

// CreateObject create new object
type CreateObject struct {
	client client.Client
	obj    runtime.Object
}

func (o *CreateObject) Execute(ctx context.Context) error {
	if err := o.client.Create(ctx, o.obj.(client.Object)); err != nil {
		return fmt.Errorf("error %q while creating object", err)
	}
	return nil
}
