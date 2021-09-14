module github.com/redhatinsights/xjoin-operator

go 1.16

require (
	github.com/elastic/go-elasticsearch/v7 v7.1.0
	github.com/go-logr/logr v0.3.0
	github.com/go-test/deep v1.0.7
	github.com/google/go-cmp v0.5.2
	github.com/google/uuid v1.1.5
	github.com/jmoiron/sqlx v1.3.1
	github.com/lib/pq v1.9.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.7.1
	github.com/riferrei/srclient v0.4.0
	github.com/spf13/viper v1.4.0
	go.uber.org/zap v1.15.0
	gopkg.in/h2non/gock.v1 v1.0.16
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)
