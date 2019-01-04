.PHONY: start stop clear

start:
	export CHANGE_MINIKUBE_NONE_USER=true
	sudo minikube start -v 5 --vm-driver=none \
	--kubernetes-version=v1.13.0 \
	--extra-config=kubelet.container-runtime=remote \
	--extra-config=kubelet.container-runtime-endpoint=unix:///var/run/singularity.sock \
	--extra-config=kubelet.image-service-endpoint=unix:///var/run/singularity.sock

stop:
	sudo minikube stop

clear:
	sudo minikube delete
	sudo rm -rf .minikube
	sudo rm -rf .kube
	sudo rm -rf /etc/kubernetes
	sudo rm -rf /var/lib/minikube/certs/
