package v1alpha1

import (
	"reflect"
	"testing"
)

func TestKruizeSpecUsesGoStyleImageAndClusterFieldNames(t *testing.T) {
	typeOfSpec := reflect.TypeOf(KruizeSpec{})

	for fieldName, jsonName := range map[string]string{
		"ClusterType":     "cluster_type",
		"AutotuneImage":   "autotune_image",
		"AutotuneUIImage": "autotune_ui_image",
		"OptimizerImage":  "optimizer_image,omitempty",
	} {
		field, found := typeOfSpec.FieldByName(fieldName)
		if !found {
			t.Errorf("KruizeSpec is missing %s", fieldName)
			continue
		}
		if actual := field.Tag.Get("json"); actual != jsonName {
			t.Errorf("%s JSON tag = %q, want %q", fieldName, actual, jsonName)
		}
	}
}
