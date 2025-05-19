package controller

import (
	"context"
	"sort"

	sdvv1alpha1 "github.com/HUFS-MCO/non_rt/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NonRTMonitoringReconciler reconciles a NonRTMonitoring object
type NonRTMonitoringReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sdv.non-rt,resources=nonrtmonitorings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sdv.non-rt,resources=nonrtmonitorings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sdv.non-rt,resources=nonrtmonitorings/finalizers,verbs=update

type Task struct {
	Name        string
	Scenario    string
	Criticality string
}

func (r *NonRTMonitoringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var nonrtList sdvv1alpha1.NonRTMonitoringList
	if err := r.List(ctx, &nonrtList); err != nil {
		logger.Error(err, "unable to list NonRTMonitoring resources")
		return ctrl.Result{}, err
	}

	scenarioMap := make(map[string][]Task)
	for _, item := range nonrtList.Items {
		scenario := item.Spec.Scenario
		criticality := item.Spec.Criticality
		scenarioMap[scenario] = append(scenarioMap[scenario], Task{
			Name:        item.Name,
			Scenario:    scenario,
			Criticality: criticality,
		})
	}

	type GroupPriority struct {
		Scenario    string
		Criticality string
		Tasks       []Task
	}

	var groups []GroupPriority
	for scenario, tasks := range scenarioMap {
		maxCrit := "A"
		for _, t := range tasks {
			if t.Criticality > maxCrit {
				maxCrit = t.Criticality
			}
		}
		groups = append(groups, GroupPriority{
			Scenario:    scenario,
			Criticality: maxCrit,
			Tasks:       tasks,
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Criticality > groups[j].Criticality
	})

	for _, group := range groups {
		logger.Info("Processing Non-RT task group", "Scenario", group.Scenario, "Criticality", group.Criticality)
		for _, task := range group.Tasks {
			deployment := generateDeployment(task)
			if err := r.Client.Create(ctx, deployment); err != nil {
				logger.Error(err, "failed to create Deployment", "Task", task.Name)
				continue
			}
			logger.Info("Deployment created", "Task", task.Name)
		}
	}

	return ctrl.Result{}, nil
}

func generateDeployment(task Task) *appsv1.Deployment {
	labels := map[string]string{
		"app": task.Name,
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      task.Name + "-deployment",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointerTo(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  task.Name,
						Image: "nginx", // 임시 이미지
					}},
				},
			},
		},
	}
}

func pointerTo(i int32) *int32 {
	return &i
}

func (r *NonRTMonitoringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sdvv1alpha1.NonRTMonitoring{}).
		Complete(r)
}