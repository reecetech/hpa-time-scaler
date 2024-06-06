package main

import (
	"context"
	"fmt"
	"log"
	"time"
	_ "time/tzdata" // Required for time.LoadLocation to work with custom timezones https://stackoverflow.com/questions/59044243/timezones-failing-to-load-in-go-1-13

	"github.com/caarlos0/env/v11"
	v1 "k8s.io/api/core/v1"
	v1events "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type config struct {
	ScaleUpTime       string `env:"SCALE_UP_TIME" envDefault:"05:00"`
	ScaleDownTime     string `env:"SCALE_DOWN_TIME" envDefault:"18:00"`
	Timezone          string `env:"TIMEZONE" envDefault:"UTC"`
	ScaleUpReplicas   int    `env:"SCALE_UP_REPLICAS" envDefault:"2"`
	ScaleDownReplicas int    `env:"SCALE_DOWN_REPLICAS" envDefault:"1"`
	HPAName           string `env:"HPA_NAME,required"`
	Namespace         string `env:"NAMESPACE,required"`
	LocalRun          bool   `env:"LOCAL_RUN" envDefault:"false"`
}

func main() {
	// Get the Envionment Configuration
	cfg := config{}
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatalf("Error parsing environment configuration: %v", err)
	}

	timeLocation, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("Error loading timezone: %v", err)
	}
	log.Printf("Timezone: %v", timeLocation)
	currentTime := time.Now().In(timeLocation)
	log.Printf("Current time: %v", currentTime)

	// Check if we are in the scale up time
	parsedScaleUpTime, err := time.Parse("15:04", cfg.ScaleUpTime)
	if err != nil {
		log.Fatalf("Error parsing scale up time: %v", err)
	}
	parsedScaleDownTime, err := time.Parse("15:04", cfg.ScaleDownTime)
	if err != nil {
		log.Fatalf("Error parsing scale down time: %v", err)
	}

	// Replace the date with the current date
	parsedScaleUpTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), parsedScaleUpTime.Hour(), parsedScaleUpTime.Minute(), 0, 0, timeLocation)
	parsedScaleDownTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), parsedScaleDownTime.Hour(), parsedScaleDownTime.Minute(), 0, 0, timeLocation)

	log.Printf("Current Schedule: %v - %v", parsedScaleUpTime, parsedScaleDownTime)

	// Determine what time we're in
	if parsedScaleDownTime.Before(parsedScaleUpTime) {
		if currentTime.After(parsedScaleUpTime) || currentTime.Before(parsedScaleDownTime) {
			// Scale up
			setMinReplicas(int32(cfg.ScaleUpReplicas), cfg)
		} else {
			// Scale down
			setMinReplicas(int32(cfg.ScaleDownReplicas), cfg)
		}
	} else {
		if currentTime.After(parsedScaleUpTime) && currentTime.Before(parsedScaleDownTime) {
			// Scale up
			setMinReplicas(int32(cfg.ScaleUpReplicas), cfg)
		} else {
			// Scale down
			setMinReplicas(int32(cfg.ScaleDownReplicas), cfg)
		}
	}

}

func setMinReplicas(replicas int32, cfg config) {
	// Create the Kubernetes client
	clientset, err := kubernetes.NewForConfig(getKubeConfig(cfg))
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %v", err)
	}

	// Get the current HPA
	hpa, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(cfg.Namespace).Get(context.TODO(), cfg.HPAName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Error getting HPA (%v) in namespace %v: %v", cfg.HPAName, cfg.Namespace, err)
	}

	// If the current min replicas is the same as the new min replicas, do nothing
	if *hpa.Spec.MinReplicas == replicas {
		log.Printf("HPA %v already has min replicas set to %v", cfg.HPAName, replicas)
		return
	}

	// Set the minimum replicas
	log.Printf("Setting min replicas to %v", replicas)
	patchReplicas := []byte(fmt.Sprintf(`{"spec": {"minReplicas": %v}}`, replicas))
	_, err = clientset.AutoscalingV2().HorizontalPodAutoscalers(cfg.Namespace).Patch(context.TODO(), cfg.HPAName, types.StrategicMergePatchType, patchReplicas, metav1.PatchOptions{})
	if err != nil {
		log.Fatalf("Error updating HPA: %v", err)
	}

	// Set an event on the HPA to indicate the change
	clientset.EventsV1().Events(cfg.Namespace).Create(context.TODO(), &v1events.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hpa-time-scaler",
			Namespace: cfg.Namespace,
		},
		EventTime:           metav1.MicroTime{Time: time.Now()},
		ReportingController: "hpa-time-scaler",
		ReportingInstance:   "hpa-time-scaler",
		Action:              "Scaled",
		Regarding: v1.ObjectReference{
			Kind:      "HorizontalPodAutoscaler",
			Namespace: cfg.Namespace,
			Name:      cfg.HPAName,
			UID:       hpa.UID,
		},
		Reason: "HPATimeScaleEvent",
		Note:   fmt.Sprintf("hpa-time-scaler set minReplicas to %d", replicas),
		Type:   "Normal",
	}, metav1.CreateOptions{FieldManager: "hpa-time-scaler"})
}

func getKubeConfig(cfg config) *rest.Config {
	if cfg.LocalRun {
		// We're running locally, so use the kubeconfig file
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)
		config, err := kubeConfig.ClientConfig()
		if err != nil {
			log.Fatalf("Error building kubeconfig: %v", err)
		}

		// return the config
		return config
	}

	// We're running in the cluster, so use the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	return config
}
