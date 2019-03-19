package client

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1alpha1"
	"github.com/open-policy-agent/frameworks/constraint/pkg/types"
	"github.com/pkg/errors"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8schema "k8s.io/apimachinery/pkg/runtime/schema"
)

var ctx = context.Background()

func newConstraintTemplate(name, rego string) *v1alpha1.ConstraintTemplate {
	return &v1alpha1.ConstraintTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ConstraintTemplateSpec{
			CRD: v1alpha1.CRD{
				Spec: v1alpha1.CRDSpec{
					Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
						Kind:   name,
						Plural: strings.ToLower(name) + "s",
					},
					Validation: &v1alpha1.Validation{
						OpenAPIV3Schema: &apiextensionsv1beta1.JSONSchemaProps{
							Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
								"expected": apiextensionsv1beta1.JSONSchemaProps{Type: "string"},
							},
						},
					},
				},
			},
			Targets: map[string]v1alpha1.Target{
				"TestTarget": v1alpha1.Target{Rego: rego},
			},
		},
	}
}

func e(s string, r types.Responses) error {
	return fmt.Errorf("%s\n%s", s, r.TraceDump())
}

func newConstraint(kind, name string, params map[string]string) *unstructured.Unstructured {
	c := &unstructured.Unstructured{}
	c.SetGroupVersionKind(k8schema.GroupVersionKind{
		Group:   "constraints.gatekeeper.sh",
		Version: "v1alpha1",
		Kind:    kind,
	})
	c.SetName(name)
	unstructured.SetNestedStringMap(c.Object, params, "spec", "parameters")
	return c
}

var tests = map[string]func(Client) error{

	"Add Template": func(c Client) error {
		_, tr := c.AddTemplate(ctx, newConstraintTemplate("Foo", `package foo
deny[{"msg": "DENIED", "details": {}}] {
	"always" == "always"
}`))
		return errors.Wrap(tr.Error(), "AddTemplate")
	},

	"Deny All": func(c Client) error {
		_, tr := c.AddTemplate(ctx, newConstraintTemplate("Foo", `package foo
deny[{"msg": "DENIED", "details": {}}] {
	"always" == "always"
}`))
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddTemplate")
		}
		cstr := newConstraint("Foo", "ph", nil)
		if tr := c.AddConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddConstraint")
		}
		rsps, tr := c.Review(ctx, targetData{Name: "Sara", ForConstraint: "Foo"})
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "Review")
		}
		if len(rsps) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps.Results()) != 1 {
			return e("Bad number of results", rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Constraint, cstr) {
			return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps.Results()[0].Constraint), spew.Sdump(cstr)), rsps)
		}
		if rsps.Results()[0].Msg != "DENIED" {
			return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps.Results()[0].Msg), rsps)
		}
		return nil
	},

	"Deny All Audit x2": func(c Client) error {
		_, tr := c.AddTemplate(ctx, newConstraintTemplate("Foo", `package foo
	deny[{"msg": "DENIED", "details": {}}] {
		"always" == "always"
	}`))
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddTemplate")
		}
		cstr := newConstraint("Foo", "ph", nil)
		if tr := c.AddConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddConstraint")
		}
		obj := &targetData{Name: "Sara", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddData")
		}
		obj2 := &targetData{Name: "Max", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj2); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddDataX2")
		}
		rsps, tr := c.Audit(ctx)
		if tr != nil {
			return errors.Wrap(tr.Error(), "Audit")
		}
		if len(rsps) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps.Results()) != 2 {
			return e("Bad number of results", rsps)
		}
		for _, r := range rsps.Results() {
			if !reflect.DeepEqual(r.Constraint, cstr) {
				return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps.Results()[0].Constraint), spew.Sdump(cstr)), rsps)
			}
			if r.Msg != "DENIED" {
				return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps.Results()[0].Msg), rsps)
			}
		}
		return nil
	},

	"Deny All Audit": func(c Client) error {
		_, tr := c.AddTemplate(ctx, newConstraintTemplate("Foo", `package foo
	deny[{"msg": "DENIED", "details": {}}] {
		"always" == "always"
	}`))
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddTemplate")
		}
		cstr := newConstraint("Foo", "ph", nil)
		if tr := c.AddConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddConstraint")
		}
		obj := &targetData{Name: "Sara", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj); tr != nil {
			return errors.Wrap(tr.Error(), "AddData")
		}
		rsps, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "Audit")
		}
		if len(rsps) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps.Results()) != 1 {
			return e("Bad number of results", rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Constraint, cstr) {
			return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps.Results()[0].Constraint), spew.Sdump(cstr)), rsps)
		}
		if rsps.Results()[0].Msg != "DENIED" {
			return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps.Results()[0].Msg), rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Resource, obj) {
			return e(fmt.Sprintf("Resource %s != %s", spew.Sdump(rsps.Results()[0].Resource), spew.Sdump(obj)), rsps)
		}
		return nil
	},

	"Remove Data": func(c Client) error {
		_, tr := c.AddTemplate(ctx, newConstraintTemplate("Foo", `package foo
	deny[{"msg": "DENIED", "details": {}}] {
		"always" == "always"
	}`))
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddTemplate")
		}
		cstr := newConstraint("Foo", "ph", nil)
		if tr := c.AddConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddConstraint")
		}
		obj := &targetData{Name: "Sara", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddData")
		}
		obj2 := &targetData{Name: "Max", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj2); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddDataX2")
		}
		rsps, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "Audit")
		}
		if len(rsps) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps.Results()) != 2 {
			return e("Bad number of results", rsps)
		}
		for _, r := range rsps.Results() {
			if !reflect.DeepEqual(r.Constraint, cstr) {
				return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps.Results()[0].Constraint), spew.Sdump(cstr)), rsps)
			}
			if r.Msg != "DENIED" {
				return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps.Results()[0].Msg), rsps)
			}
		}

		if tr := c.RemoveData(ctx, obj2); tr.Error() != nil {
			return tr.Error()
		}
		rsps2, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return tr.Error()
		}
		if len(rsps2) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps2.Results()) != 1 {
			return e("Bad number of results", rsps2)
		}
		if !reflect.DeepEqual(rsps2.Results()[0].Constraint, cstr) {
			return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps2.Results()[0].Constraint), spew.Sdump(cstr)), rsps2)
		}
		if rsps2.Results()[0].Msg != "DENIED" {
			return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps2.Results()[0].Msg), rsps2)
		}
		if !reflect.DeepEqual(rsps2.Results()[0].Resource, obj) {
			return e(fmt.Sprintf("Resource %s != %s", spew.Sdump(rsps2.Results()[0].Resource), spew.Sdump(obj)), rsps2)
		}
		return nil
	},

	"Remove Constraint": func(c Client) error {
		_, tr := c.AddTemplate(ctx, newConstraintTemplate("Foo", `package foo
	deny[{"msg": "DENIED", "details": {}}] {
		"always" == "always"
	}`))
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddTemplate")
		}
		cstr := newConstraint("Foo", "ph", nil)
		if tr := c.AddConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddConstraint")
		}
		obj := &targetData{Name: "Sara", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddData")
		}
		rsps, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "Audit")
		}
		if len(rsps) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps.Results()) != 1 {
			return e("Bad number of results", rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Constraint, cstr) {
			return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps.Results()[0].Constraint), spew.Sdump(cstr)), rsps)
		}
		if rsps.Results()[0].Msg != "DENIED" {
			return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps.Results()[0].Msg), rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Resource, obj) {
			return e(fmt.Sprintf("Resource %s != %s", spew.Sdump(rsps.Results()[0].Resource), spew.Sdump(obj)), rsps)
		}

		if tr := c.RemoveConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "RemoveConstraint")
		}
		rsps2, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AuditX2")
		}
		if len(rsps2.Results()) != 0 {
			return e("Responses returned", rsps2)
		}
		return nil
	},

	"Remove Template": func(c Client) error {
		tmpl := newConstraintTemplate("Foo", `package foo
	deny[{"msg": "DENIED", "details": {}}] {
		"always" == "always"
	}`)
		_, tr := c.AddTemplate(ctx, tmpl)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddTemplate")
		}
		cstr := newConstraint("Foo", "ph", nil)
		if tr := c.AddConstraint(ctx, cstr); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddConstraint")
		}
		obj := &targetData{Name: "Sara", ForConstraint: "Foo"}
		if tr := c.AddData(ctx, obj); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AddData")
		}
		rsps, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "Audit")
		}
		if len(rsps) == 0 {
			return errors.New("No responses returned")
		}
		if len(rsps.Results()) != 1 {
			return e("Bad number of results", rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Constraint, cstr) {
			return e(fmt.Sprintf("Constraint %s != %s", spew.Sdump(rsps.Results()[0].Constraint), spew.Sdump(cstr)), rsps)
		}
		if rsps.Results()[0].Msg != "DENIED" {
			return e(fmt.Sprintf("res.Msg = %s; wanted DENIED", rsps.Results()[0].Msg), rsps)
		}
		if !reflect.DeepEqual(rsps.Results()[0].Resource, obj) {
			return e(fmt.Sprintf("Resource %s != %s", spew.Sdump(rsps.Results()[0].Resource), spew.Sdump(obj)), rsps)
		}

		if tr := c.RemoveTemplate(ctx, tmpl); tr.Error() != nil {
			return errors.Wrap(tr.Error(), "RemoveTemplate")
		}
		rsps2, tr := c.Audit(ctx)
		if tr.Error() != nil {
			return errors.Wrap(tr.Error(), "AuditX2")
		}
		if len(rsps2.Results()) != 0 {
			return e("Responses returned", rsps2)
		}
		return nil
	},
}

// TODO: Test metadata, test idempotence, test multiple targets