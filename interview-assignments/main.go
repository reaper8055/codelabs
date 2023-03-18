package main

import (
	"context"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func listPods(clientset *kubernetes.Clientset, namespace, deploymentName string) []corev1.Pod {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("listPods: %s", err.Error())
	}
	return pods.Items
}

func deletePods(clientset *kubernetes.Clientset, namespace, deploymentName string) {
	if namespace != "kube-system" {
		for _, pod := range listPods(clientset, namespace, deploymentName) {
			if pod.Name == "podkiller" {
				continue
			}
			if err := clientset.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
				log.Printf("failed to delete pod %s: %v\n", pod.Name, err)
				continue
			} else {
				log.Printf("Deleted pod %s\n", pod.Name)
			}
		}
	}
}

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("creating in-cluster config: %s\n", err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("creating clientset config: %s\n", err.Error())
	}

	deploymentsListWatcher := cache.NewListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", metav1.NamespaceAll, fields.Everything())
	deploymentsInformer := cache.NewSharedInformer(deploymentsListWatcher, &v1.Deployment{}, time.Minute)

	deploymentsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			deployment := object.(*v1.Deployment)
			listPods(clientset, deployment.Namespace, deployment.Name)
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			oldDeployment := oldObject.(*v1.Deployment)
			newDeployment := newObject.(*v1.Deployment)

			if newDeployment.Generation != oldDeployment.Generation {
				return
			}

			oldPods := make(map[string]bool)
			for _, pod := range listPods(clientset, oldDeployment.Namespace, oldDeployment.Name) {
				oldPods[pod.Name] = true
			}

			for _, pod := range listPods(clientset, newDeployment.Namespace, newDeployment.Name) {
				if _, exists := oldPods[pod.Name]; !exists {
					fmt.Printf("New pod created: %s/%s\n", newDeployment.Namespace, pod.Name)
					deletePods(clientset, newDeployment.Namespace, newDeployment.Name)
				}
			}

			if newDeployment.Spec.Replicas != nil && *newDeployment.Spec.Replicas > 0 && newDeployment.Namespace != "kube-system" {
				log.Printf("Updating replicas for %s/%s to 0", newDeployment.Namespace, newDeployment.Name)
				newDeployment.Spec.Replicas = new(int32)
				*newDeployment.Spec.Replicas = 0
				_, err := clientset.AppsV1().Deployments(newDeployment.Namespace).Update(context.TODO(), newDeployment, metav1.UpdateOptions{})
				if err != nil {
					log.Printf("error updating deployment %s: %s", newDeployment.Name, err.Error())
				}
			}
		},
	})

	stopCh := make(chan struct{})
	go func() {
		log.Println("deploymentInformer.Run(stopCh)")
		deploymentsInformer.Run(stopCh)
	}()

	// check if deploymentInformer cache synced
	if !cache.WaitForCacheSync(context.Background().Done(), deploymentsInformer.HasSynced) {
		log.Print("failed to sync deploymentInfromer cache")
	}
	fmt.Println("deploymentInformer.HasSynced returned true")

	// list existing deployments
	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("listing deployment: %s\n", err.Error())
	}
	fmt.Println("listing deployment successful")

	fmt.Println("deleting pods...")
	for _, deployment := range deployments.Items {
		fmt.Println(deployment.Namespace)
		deletePods(clientset, deployment.Namespace, deployment.Name)
	}

	select {}
}
