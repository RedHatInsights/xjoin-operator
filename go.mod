module github.com/redhatinsights/xjoin-operator

go 1.13

require (
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/elastic/go-elasticsearch/v7 v7.1.0
	github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/gofrs/uuid v3.3.0+incompatible // indirect
	github.com/google/go-cmp v0.4.0
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/jackc/pgx v3.6.2+incompatible
	github.com/lib/pq v1.9.0 // indirect
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/prometheus/client_golang v1.0.0
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/viper v1.3.2
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	sigs.k8s.io/controller-runtime v0.6.3
)
