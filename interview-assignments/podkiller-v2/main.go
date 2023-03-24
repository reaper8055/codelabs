package main

import (
	"context"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	ctx = context.TODO()
)

func isDeploymentPod(clientset *kubernetes.Clientset, podName, podNamespace string) bool {
	pod, err := clientset.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		log.Printf("error getting %s/%s: %s\n", podNamespace, podName, err.Error())
		return false
	}
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" && podNamespace != "kube-system" {
			log.Printf("%s/%s is a Deployment Pod\n", podNamespace, podName)
			return true
		}
	}
	return false
}

func deletePod(clientset *kubernetes.Clientset, podName, podNamespace string) {
	if err := clientset.CoreV1().Pods(podNamespace).Delete(ctx, podName, metav1.DeleteOptions{}); err != nil {
		log.Printf("error deleting pod: %s/%s: %s\n", podNamespace, podName, err.Error())
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

	podsListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", metav1.NamespaceAll, fields.Everything())
	podsInformer := cache.NewSharedInformer(podsListWatcher, &v1.Pod{}, time.Minute)

	_, err = podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if isDeploymentPod(clientset, pod.Name, pod.Namespace) {
				deletePod(clientset, pod.Name, pod.Namespace)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*v1.Pod)
			newPod := newObj.(*v1.Pod)

			if newPod.Generation != oldPod.Generation {
				return
			}

			deploymentPods := make(map[string]bool)
			if isDeploymentPod(clientset, oldPod.Name, oldPod.Namespace) {
				deploymentPods[oldPod.Name] = true
			}
			if isDeploymentPod(clientset, newPod.Name, newPod.Namespace) {
				if _, exists := deploymentPods[newPod.Name]; !exists {
					deletePod(clientset, newPod.Name, newPod.Namespace)
				}
			}
		},
	})

	if err != nil {
		log.Printf("error at podsInformer.AddEventHandler(): %s\n", err.Error())
	}

	stopCh := make(chan struct{})
	go func() {
		log.Println("podsInformer.Run(stopCh)")
		podsInformer.Run(stopCh)
	}()

	if !cache.WaitForCacheSync(ctx.Done(), podsInformer.HasSynced) {
		log.Println("failed to sync podsInformer")
	}

	// list all pods
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("error listing pods: %s", err.Error())
	}

	for _, pod := range pods.Items {
		if isDeploymentPod(clientset, pod.Name, pod.Namespace) {
			deletePod(clientset, pod.Name, pod.Namespace)
		}
	}

	select {}

}
