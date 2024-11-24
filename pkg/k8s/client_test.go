package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsEvictable(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "normal pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "normal-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			want: true,
		},
		{
			name: "mirror pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mirror-pod",
					Namespace: "kube-system",
					Annotations: map[string]string{
						corev1.MirrorPodAnnotationKey: "true",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			want: false,
		},
		{
			name: "daemonset pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ds-pod",
					Namespace: "kube-system",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "DaemonSet",
							Name: "my-daemonset",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			want: false,
		},
		{
			name: "terminating pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "terminating-pod",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			want: false,
		},
		{
			name: "succeeded pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "succeeded-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			want: false,
		},
		{
			name: "failed pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failed-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			want: false,
		},
		{
			name: "deployment pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy-pod",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "ReplicaSet",
							Name: "my-rs",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEvictable(tt.pod)
			if got != tt.want {
				t.Errorf("isEvictable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPodPriority(t *testing.T) {
	tests := []struct {
		name     string
		qosClass corev1.PodQOSClass
		want     PodPriority
	}{
		{
			name:     "best effort",
			qosClass: corev1.PodQOSBestEffort,
			want:     PriorityLow,
		},
		{
			name:     "burstable",
			qosClass: corev1.PodQOSBurstable,
			want:     PriorityMedium,
		},
		{
			name:     "guaranteed",
			qosClass: corev1.PodQOSGuaranteed,
			want:     PriorityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					QOSClass: tt.qosClass,
				},
			}
			got := GetPodPriority(pod)
			if got != tt.want {
				t.Errorf("GetPodPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortPodsForEviction(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "guaranteed"},
			Status:     corev1.PodStatus{QOSClass: corev1.PodQOSGuaranteed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "besteffort"},
			Status:     corev1.PodStatus{QOSClass: corev1.PodQOSBestEffort},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "burstable"},
			Status:     corev1.PodStatus{QOSClass: corev1.PodQOSBurstable},
		},
	}

	sorted := SortPodsForEviction(pods)

	// BestEffort should be first (lowest priority, evict first)
	if sorted[0].Name != "besteffort" {
		t.Errorf("first pod = %v, want besteffort", sorted[0].Name)
	}

	// Burstable should be second
	if sorted[1].Name != "burstable" {
		t.Errorf("second pod = %v, want burstable", sorted[1].Name)
	}

	// Guaranteed should be last (highest priority, evict last)
	if sorted[2].Name != "guaranteed" {
		t.Errorf("third pod = %v, want guaranteed", sorted[2].Name)
	}
}

func TestDrainResult(t *testing.T) {
	result := DrainResult{
		NodeName:    "test-node",
		Cordoned:    true,
		TotalPods:   5,
		EvictedPods: []string{"pod1", "pod2", "pod3"},
		FailedEvictions: []PodEvictionError{
			{Pod: "pod4", Error: "eviction denied"},
			{Pod: "pod5", Error: "pdb violation"},
		},
		Success: false,
	}

	if result.Success {
		t.Error("Success should be false when there are failed evictions")
	}
	if len(result.EvictedPods) != 3 {
		t.Errorf("EvictedPods = %d, want 3", len(result.EvictedPods))
	}
	if len(result.FailedEvictions) != 2 {
		t.Errorf("FailedEvictions = %d, want 2", len(result.FailedEvictions))
	}
}

func TestMigrationReasons(t *testing.T) {
	reasons := []MigrationReason{
		ReasonThermalCritical,
		ReasonMemoryPressure,
		ReasonPredictedFailure,
		ReasonManualRequest,
		ReasonPartitionRecovery,
	}

	for _, r := range reasons {
		if r == "" {
			t.Error("MigrationReason should not be empty")
		}
	}
}
