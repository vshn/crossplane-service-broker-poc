module broker

go 1.15

require (
	code.cloudfoundry.org/lager v2.0.0+incompatible
	github.com/crossplane-contrib/provider-helm v0.4.0
	github.com/crossplane/crossplane v0.14.0
	github.com/crossplane/crossplane-runtime v0.11.0
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/pivotal-cf/brokerapi/v7 v7.4.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.6.3
)
