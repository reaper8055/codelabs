# PodKiller

A lot of the code here is written by using the [examples](https://github.com/kubernetes/client-go/tree/master/examples) as mentioned in [client-go](https://github.com/kubernetes/client-go#compatibility-client-go---kubernetes-clusters) repo. And huge shoutout to [sample-controller](https://github.com/kubernetes/sample-controller) repo for some neat tricks on using watcher and informer.

## How to use this

1. Install minikube
   1. Start a new cluster with: ```minikube start```
2. Build the binary:
   1. ```cd interview-assignments/```
   2. ```export CGO_ENABLED=0; GOOS=linux go build -o ./app .```
   3. ```eval $(minikube docker-env)```
   4. ```docker build -t in-cluster:0.1.2 .``` (name:tag is important to be able to use local images.)
   5. Create a role so that the app can delete stuff: ```kubectl create clusterrolebinding default-view --clusterrole=cluster-admin --serviceaccount=default:default```
   6. Run the container as a pod by issuing ```kubectl run --rm -i podkiller --image=in-cluster:0.1.2 --namespace=kube-system`
   7. You can use `nginx.yaml` to create new deployments by issing: `kubectl apply -f nginx.yaml```


## My journey in getting this work

Ask: Delete all pods for any deployment in any namespace, except kube-system for a given k8s cluster. This should run inside the cluster as a binary.

### Delete all pods in any namespace except kube-system

The ask was to delete all pods for any deployments. That sounds staright forward, so we just list the deployments and the delete the pods associated with them.

Use the `deployment.Namespace` to filter `kube-system` out and the we should be golden, right? That's what I thought too. 

Deployment objects in kubernetes make sure that the pods have a "desired state" i.e the deployment yaml had a spec defined called "replicas" which defines this "desired state".

Even if we are successful in deleting pods associated with a deployment k8s will make sure that the desired state is reached which means we need to think of a solution that watches the deployments and keeps deleting them, maybe there's a event handler in client-go?

This code blow works but as said it doesn't do anything when k8s's controller manager spaws up new pods to reach desired state.

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

### Using deployment watcher works but doesn't delete new deployment

client-go has a method under `AppsV1().Deployments("").Watch(...)`, how I found out about this, hitting ctrl+space on vscode to find all callable methods on `Deployments("")`. And we can use `ListOptions{}` abailable under `k8s.io/apimachinery/pkg/apis/meta/v1` to fill some of the options as shown below:

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
I was so happy to see that works, but it got a problem, remember what I said about k8s trying to bring the pods to "desired state", well that happened again. Which means my use of deploymentWatcher somehow doesn't work. Or maybe k8s's way of updating pod generations is different. Looking at [this code](https://github.com/kubernetes/sample-controller/blob/master/controller.go) from sample-controller repo, there seems to be a work-queue of sorts that keeps the items and updates them as changes come by, there's also something called a `sharedIndexInfromer` which seems to be having the infromation I need. 

Basically, there needs to be a `watcher` and a `informer` by using this https://github.com/kubernetes/client-go/blob/master/tools/cache/listwatch.go like a cache to sync updates from the controller-manager and the changes made by my controller.

At this point, I'm pretty lost... So I thought of splitting the problem into smaller chunks so that I can manage them and think about combinging them later....

### Tackling one problem at a time

Here goes nothing...

#### lets get all the deployments in the cluster:

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

This one works as intended, the sleep at the end of the main in the for loop is to make sure I get stdout prints on my host console instead of the container console. We can now list all pods associated with any deployment.

Moving on to the next problem at hand...

#### now lets create a watcher

The watcher needs to watch changes in deployments made by k8s and tell us the controller manager is spinning up pods because of "relpicas" spec and we need to act upon it. So maybe we need a watcher+informer combo of somesort to accomplish this.

Just think about it right, suppose we manually went ahaed and deleted pods for a deployment say, nginx which has 4 replicas. This means there is a state change in k8s system that triggers replication-controller which might look something like "yo replication-controller, desired state as per deployment yaml (manifest) is 4 but we got zero here". And this would essentially trigger k8s controller-manager to spin up new pods to meet the desired state.

At this point, I am just thinking, I can just get the replicas to zero and that should solve the problem, right? Well, in k8s terms yes, but I still want to give this method a go so we know the how the greas move within k8s!!

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

This one works but doesn't keep track of pod generations i.e if a deployment has replicas > 0 then it doesn't delete the new generations as kubernetes controller tries to bring it up as soon as we delete it. (sad)

we need to be able to keep track of old and new generations somehow

#### keeping track of old and new generations of Pods by created by deployment

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

This is not the end...
