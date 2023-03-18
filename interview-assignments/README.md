# PodKiller

## How to use this

1. Install minikube
   1. Start a new cluster with: `minikube start`
2. Build the binary:
   1. cd interview-assignments/
   2. export CGO_ENABLED=0; GOOS=linux go build -o ./app .
   3. eval $(minikube docker-env)
   4. docker build -t in-cluster:0.1.2 . (name:tag is important to be able to use local images.)
   5. Create a role so that the app can delete stuff: `kubectl create clusterrolebinding default-view --clusterrole=cluster-admin --serviceaccount=default:default`
   6. Run the container as a pod by issuing `kubectl run --rm -i podkiller --image=in-cluster:0.1.2 --namespace=kube-system`
   7. You can use nginx.yaml to create new deployments by issing: `kubectl apply -f nginx.yaml`

## Delete all pods in any namespace except kube-system

```go
deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
 if err != nil {
  fmt.Println(err.Error())
  panic(err.Error())
 }

 for _, deployment := range deployments.Items {
  if deployment.Namespace == "kube-system" {
   continue
  }
  pods, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.TODO(), metav1.ListOptions{})
  if err != nil {
   fmt.Println(err.Error())
   panic(err.Error())
  }
  for _, pod := range pods.Items {
   err := clientset.CoreV1().Pods(deployment.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
   if err != nil {
    log.Fatalf("error deleting pod: %s -- %s", pod.Name, err.Error())
   }
   fmt.Printf("Deleted pod %s in namespace %s\n", pod.Name, deployment.Namespace)
  }
 }
 fmt.Println("Deleting complete")
```

## Using deployment watcher works but doesn't delete new deployment

```go
deploymentWatcher, err := clientset.AppsV1().Deployments("").Watch(context.TODO(), metav1.ListOptions{
  Watch:           true,
  TimeoutSeconds:  &timeout,
  LabelSelector:   "app",
  FieldSelector:   "metadata.namespace!=kube-system",
  ResourceVersion: "0",
 })
 if err != nil {
  log.Fatalf("error setting up deploymentWatcher: %s\n", err.Error())
 }

 for event := range deploymentWatcher.ResultChan() {
  switch event.Type {
  case watch.Added, watch.Modified, watch.Deleted:
   deployment, ok := event.Object.(*v1.Deployment)
   if !ok {
    log.Fatalf("error getting event oject for deployment: %s\n", err.Error())
   }
   if deployment.Namespace == "kube-system" {
    continue
   }
   pods, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.TODO(), metav1.ListOptions{
    LabelSelector: fmt.Sprintf("app=%s", deployment.Name),
   })
   if err != nil {
    log.Fatalf("error listing pods in %s namespace: %s\n", deployment.Namespace, err.Error())
   }
   for _, pod := range pods.Items {
    fmt.Println(pod.Name)
    fmt.Println(deployment.Name)
    fmt.Println(deployment.Namespace)
    if err := clientset.CoreV1().Pods(deployment.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
     log.Fatalf("error listing pods in %s namespace: %s\n", deployment.Namespace, err.Error())
    }
    fmt.Printf("deleted Pod: %s in namespace %s\n", pod.Name, deployment.Namespace)
   }
   fmt.Printf("Deleted all pods associated with deployment %s in namespace %s\n", deployment.Name, deployment.Namespace)
  case watch.Error:
   log.Fatalf("error with deploymentWatcher\n")
  default:
   log.Fatalf("Unexpected event type encountered in deployment watcher: %s\n", event.Type)
  }
 }
```

## Tackling one problem at a time

- lets get all the deployments in the cluster:

```go
package main

import (
 "context"
 "fmt"
 "log"
 "time"

 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
 "k8s.io/client-go/kubernetes"
 "k8s.io/client-go/rest"
)

func listPods(clientset *kubernetes.Clientset, namespace, labelSelector string) {
 pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
 if err != nil {
  log.Fatalf("error listing pods: %s", err.Error())
 }
 for _, pod := range pods.Items {
  fmt.Println(pod.Name)
 }
}

func main() {
 // creates the in-cluster config
 config, err := rest.InClusterConfig()
 if err != nil {
  log.Fatalf("error creating in-cluster config: %s\n", err.Error())
 }

 // creates the clientset
 clientset, err := kubernetes.NewForConfig(config)
 if err != nil {
  log.Fatalf("error creating clientset config: %s\n", err.Error())
 }

 // list existing deployments
 deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
 if err != nil {
  log.Fatalf("error listing deployment: %s\n", err.Error())
 }

 for _, deployment := range deployments.Items {
  fmt.Println(deployment.Name)
  listPods(clientset, deployment.Namespace, fmt.Sprintf("app=%s", deployment.Name))
  time.Sleep(10 * time.Second)
 }
}
```

- now lets create a watcher

```go
deploymentsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
  AddFunc: func(object interface{}) {
   deployment := object.(*v1.Deployment)
   listPods(clientset, deployment.Namespace, deployment.Name)
  },
  UpdateFunc: func(oldObject, newObject interface{}) {
   newDeployment := newObject.(*v1.Deployment)
   listPods(clientset, newDeployment.Namespace, newDeployment.Name)
  },
 })
```

this one works but doesn't keep track of pod generations i.e if a deployment has replicas > 0 then it doesn't delete the new generations as kubernetes controller tries to bring it up as soon as we delete it. (sad)

we need to be able to keep track of old and new generations somehow

- keeping track of old and new generations of Pods by created by deployment

```go
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
```
