This package contains the code to manage pipeline components. A component is a piece of the pipeline that needs to be 
managed by CRUD operations e.g. a Kafka Connector.

`manager.go` contains the logic to perform the CRUD operations on a set of components.

`custodian.go` contains cleanup logic to remove orphaned components.

The remaining files are component definitions.