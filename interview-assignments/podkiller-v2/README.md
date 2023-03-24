# Feedback on podkiller-v1

1. Use a watcher on pods to handle new generations of pods coming up.
2. Handle each pod concurrently using go routines.
3. Use ownerReference for checking if a pod belongs to a deployment. 

## k8s sig suggestions:

When a deployment creates a new generation of pods, the new pods will have a different name and a different generation number than the old pods. Therefore, if we only use a watcher on Pod objects, we may not be able to distinguish between the old and new pods, and we may accidentally delete the new pods that were created to replace the old ones.

One way to handle this situation is to use the owner references of the pods to determine if they belong to the deployment. The owner references will link the pods to the Deployment or ReplicaSet that created them, and we can use this information to make sure we only delete the old pods that were replaced by the new ones.

However, using owner references can be tricky, as they can be subject to change depending on the Kubernetes object being used. In particular, owner references can change when a Deployment scales up or down, or when a Deployment is updated with a new version of the pod template.

Also, When a Kubernetes Deployment scales up or down, or when it is updated with a new version of the pod template, the owner references of the Pod objects created by the deployment can change.

The owner references of a pod identify the Kubernetes object that created the pod, and they are used to establish a hierarchical relationship between the pod and its creator. For example, a Pod object created by a Deployment will have an owner reference to the Deployment object.

However, when the deployment is scaled up or down, or when it is updated with a new version of the pod template, the owner references of the existing pods may need to be updated to reflect the new state of the deployment. This can happen when a new generation of pods is created, or when existing pods are deleted or replaced.

When the owner references of a pod change, this can affect how the pod is managed by Kubernetes. For example, if a pod has an owner reference to a Deployment object, and the deployment is deleted, the pod will be deleted automatically by Kubernetes. If the owner reference is changed to a different object, the pod may no longer be deleted automatically when the original owner is deleted.

Therefore, when using owner references to determine if a pod belongs to a deployment, it is important to be aware of how the owner references can change over time, and to take this into account when designing your application.

Therefore, to handle new generations of pods correctly, it is generally recommended to use a watcher on the Deployment or ReplicaSet objects themselves, rather than on the Pod objects. This will allow us to track changes to the deployment or replica set, including the creation of new generations of pods, and take appropriate action based on the owner references of the pods.
