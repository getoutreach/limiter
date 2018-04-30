package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// test syncHandler

// test enqueueNamespace

// test handleObject

// test newLimitRange
func TestNewLimitRange(t *testing.T) {
	config = limitRangeConfig{
		container: limitRangeItem{
			min: resourceList{
				memory: "10Mi",
			},
			request: resourceList{
				cpu:    "10m",
				memory: "100Mi",
			},
			limit: resourceList{
				memory: "500Mi",
			},
			max: resourceList{
				memory: "1000Mi",
			},
		},
	}
	expected := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "limits",
			Namespace: "foo",
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Default: corev1.ResourceList{
						"memory": resource.MustParse(config.container.limit.memory),
					},
					DefaultRequest: corev1.ResourceList{
						"cpu":    resource.MustParse(config.container.request.cpu),
						"memory": resource.MustParse(config.container.request.memory),
					},
					Min: corev1.ResourceList{
						"memory": resource.MustParse(config.container.min.memory),
					},
					Max: corev1.ResourceList{
						"memory": resource.MustParse(config.container.max.memory),
					},
					Type: "Container",
				},
				{
					Min:  corev1.ResourceList{},
					Max:  corev1.ResourceList{},
					Type: "Pod",
				},
			},
		},
	}
	ns := &corev1.Namespace{}
	ns.SetName("foo")
	actual := NewLimitRange(ns)

	assert.Equal(t, actual, expected, "The two LimitRanges should be the same.")
}
