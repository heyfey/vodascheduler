module github.com/heyfey/vodascheduler

go 1.13

require (
	github.com/gorilla/mux v1.8.0
	github.com/heyfey/munkres v0.0.0-20210705070356-d4f16a25250d
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/kubeflow/common v0.3.3
	github.com/kubeflow/mpi-operator v0.3.0
	github.com/prometheus/client_golang v1.5.1
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	k8s.io/api v0.16.15
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog/v2 v2.0.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.15.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.15.10
	k8s.io/client-go => k8s.io/client-go v0.15.10
)
