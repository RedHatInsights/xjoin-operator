package controllers_test

import (
	"bytes"
	"context"
	"fmt"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"github.com/redhatinsights/xjoin-operator/controllers/index"
	"os"
	"strings"
	"text/template"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type XJoinIndexPipelineTestReconciler struct {
	Namespace                  string
	Name                       string
	Version                    string
	AvroSchemaFileName         string
	ElasticsearchIndexFileName string
	CustomSubgraphImages       []v1alpha1.CustomSubgraphImage
	K8sClient                  client.Client
	DataSources                []DataSource
	createdIndexPipeline       v1alpha1.XJoinIndexPipeline
	ApiCurioResponseFilename   string
}

type DataSource struct {
	Name                     string
	Version                  string
	ApiCurioResponseFilename string
}

func (x *XJoinIndexPipelineTestReconciler) getNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: x.Namespace,
		Name:      x.GetName(),
	}
}

func (x *XJoinIndexPipelineTestReconciler) GetName() string {
	return x.Name + "." + x.Version
}

func (x *XJoinIndexPipelineTestReconciler) ReconcileNew() v1alpha1.XJoinIndexPipeline {
	x.registerNewMocks()
	x.createValidIndexPipeline()
	result := x.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))
	indexPipelineLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	Eventually(func() bool {
		err := x.K8sClient.Get(context.Background(), indexPipelineLookupKey, &x.createdIndexPipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
	return x.createdIndexPipeline
}

func (x *XJoinIndexPipelineTestReconciler) ReconcileUpdated(params UpdatedMocksParams) v1alpha1.XJoinIndexPipeline {
	x.registerUpdatedMocks(params)
	result := x.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))
	indexPipelineLookup := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	Eventually(func() bool {
		err := x.K8sClient.Get(context.Background(), indexPipelineLookup, &x.createdIndexPipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
	return x.createdIndexPipeline
}

func (x *XJoinIndexPipelineTestReconciler) ReconcileValid() v1alpha1.XJoinIndexPipeline {
	//mock the validation controller by directly setting the status to valid
	indexPipeline := &v1alpha1.XJoinIndexPipeline{}
	Eventually(func() bool {
		err := x.K8sClient.Get(context.Background(), x.getNamespacedName(), indexPipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	indexPipeline.Status.ValidationResponse = validation.ValidationResponse{
		Result: index.Valid,
	}
	indexPipeline.Status.Active = true
	err := x.K8sClient.Status().Update(context.Background(), indexPipeline)
	Expect(err).ToNot(HaveOccurred())

	//reconcile the indexpipeline
	x.registerValidMocks()
	result := x.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	//get the indexpipeline
	Eventually(func() bool {
		err = x.K8sClient.Get(context.Background(), x.getNamespacedName(), indexPipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	Expect(indexPipeline.Status.ValidationResponse.Message).To(Equal(""))
	Expect(indexPipeline.Status.ValidationResponse.Result).To(Equal(index.Valid))

	x.createdIndexPipeline = *indexPipeline
	return x.createdIndexPipeline
}

func (x *XJoinIndexPipelineTestReconciler) ReconcileDelete() {
	x.registerDeleteMocks()
	result := x.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 0}))

	indexPipelineList := v1alpha1.XJoinIndexPipelineList{}
	err := x.K8sClient.List(context.Background(), &indexPipelineList, client.InNamespace(x.Namespace))
	checkError(err)
	Expect(indexPipelineList.Items).To(HaveLen(0))
}

func (x *XJoinIndexPipelineTestReconciler) reconcile() reconcile.Result {
	ctx := context.Background()
	xjoinIndexPipelineReconciler := x.newXJoinIndexPipelineReconciler()
	indexLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	result, err := xjoinIndexPipelineReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: indexLookupKey})
	checkError(err)
	return result
}

func (x *XJoinIndexPipelineTestReconciler) registerDeleteMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//connector mocks
	httpmock.RegisterResponder(
		"GET",
		"http://connect-connect-api."+x.Namespace+".svc:8083/connectors/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(404, `{}`))

	//gql schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"/versions/1",
		httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"/versions/latest",
		httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder(
		"DELETE",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(200, `{}`))

	//graphql schema state update mocks
	getMetaResponder, err := httpmock.NewJsonResponder(200, map[string]interface{}{
		"state":  "ENABLED",
		"labels": x.parseGraphQLSchemaLabels(x.GetName(), nil)})
	checkError(err)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/meta",
		getMetaResponder)

	//elasticsearch index mocks
	httpmock.RegisterResponder(
		"HEAD",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder(
		"DELETE",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(200, `{}`))

	//avro schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"DELETE",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/versions",
		httpmock.NewStringResponder(404, `{}`))

	for _, customImage := range x.CustomSubgraphImages {
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-"+customImage.Name+"."+x.Version+"/versions",
			httpmock.NewStringResponder(200, `{}`).Once())

		httpmock.RegisterResponder(
			"DELETE",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-"+customImage.Name+"."+x.Version,
			httpmock.NewStringResponder(200, `{}`).Once())
	}

	for _, dataSource := range x.DataSources {
		response, err := os.ReadFile("./test/data/apicurio/" + dataSource.ApiCurioResponseFilename + ".json")
		checkError(err)
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+dataSource.Name+"."+dataSource.Version+"-value/versions/latest",
			httpmock.NewStringResponder(200, string(response)))

		httpmock.RegisterResponder(
			"GET",
			"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline."+x.GetName(),
			httpmock.NewStringResponder(200, "{}").Once())

		httpmock.RegisterResponder(
			"DELETE",
			"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline."+x.GetName(),
			httpmock.NewStringResponder(200, "{}").Once())
	}
}

func (x *XJoinIndexPipelineTestReconciler) registerNewMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//elasticsearch index mocks
	indexExistsResponder := httpmock.NewStringResponder(200, `{}`)
	httpmock.RegisterResponder(
		"HEAD",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(404, `{}`).Then(indexExistsResponder))

	httpmock.RegisterResponder(
		"PUT",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(201, `{}`))

	esIndexResponse := x.parseElasticsearchIndex("get-index-response")
	httpmock.RegisterResponder(
		"GET",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(200, esIndexResponse))

	//avro schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(404, `{"message":"No version '1' found for artifact with ID 'xjoinindexpipelinepipeline.`+x.GetName()+`-value' in group 'null'.","error_code":40402}`))

	httpmock.RegisterResponder(
		"POST",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions",
		httpmock.NewStringResponder(200, `{"createdBy":"","createdOn":"2022-07-27T17:28:11+0000","modifiedBy":"","modifiedOn":"2022-07-27T17:28:11+0000","id":1,"version":1,"type":"AVRO","globalId":1,"state":"ENABLED","groupId":"null","contentId":1,"references":[]}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/schemas/ids/1",
		httpmock.NewStringResponder(200, `{"schema":"{\"name\":\"Value\",\"namespace\":\"xjoindatasourcepipeline.`+x.GetName()+`\"}","schemaType":"AVRO","references":[]}`))

	//graphql schema state update mocks
	getMetaResponder, err := httpmock.NewJsonResponder(200, map[string]interface{}{
		"state":  "ENABLED",
		"labels": x.parseGraphQLSchemaLabels(x.GetName(), nil)})
	checkError(err)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/meta",
		getMetaResponder)

	httpmock.RegisterMatcherResponder(
		"PUT",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/state",
		httpmock.BodyContainsString(`"state":"DISABLED"`).WithName("DisabledState"),
		httpmock.NewStringResponder(200, `{}`))

	//graphql schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/versions",
		httpmock.NewStringResponder(404, `{}`).Then(httpmock.NewStringResponder(200, `{}`)))

	httpmock.RegisterResponder(
		"POST",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts",
		httpmock.NewStringResponder(201, `{}`))

	httpmock.RegisterResponder(
		"PUT",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/meta",
		httpmock.NewStringResponder(200, `{}`))

	for _, customImage := range x.CustomSubgraphImages {
		customImageFullName := "test-index-pipeline-" + customImage.Name + "." + x.Version
		//custom subgraph graphql schema mocks
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/versions",
			httpmock.NewStringResponder(404, `{}`).Then(httpmock.NewStringResponder(200, `{}`)))

		httpmock.RegisterResponder(
			"POST",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts",
			httpmock.NewStringResponder(201, `{}`))

		httpmock.RegisterResponder(
			"PUT",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/meta",
			httpmock.NewStringResponder(200, `{}`))

		//graphql schema state update mocks
		getMetaResponder, err = httpmock.NewJsonResponder(200, map[string]interface{}{
			"state":  "ENABLED",
			"labels": x.parseGraphQLSchemaLabels(customImageFullName, nil)})
		checkError(err)
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/meta",
			getMetaResponder)

		httpmock.RegisterMatcherResponder(
			"PUT",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/state",
			httpmock.BodyContainsString(`"state":"DISABLED"`).WithName("DisabledState"),
			httpmock.NewStringResponder(200, `{}`))
	}

	for _, dataSource := range x.DataSources {
		response, err := os.ReadFile("./test/data/apicurio/" + dataSource.ApiCurioResponseFilename + ".json")
		checkError(err)
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+dataSource.Name+"."+dataSource.Version+"-value/versions/latest",
			httpmock.NewStringResponder(200, string(response)))

		httpmock.RegisterResponder(
			"GET",
			"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline."+x.GetName(),
			httpmock.NewStringResponder(404, "{}").Once())

		httpmock.RegisterResponder(
			"PUT",
			"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline."+x.GetName(),
			httpmock.NewStringResponder(200, "{}"))
	}

	response := x.parseResponseFile()
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, response))
}

func (x *XJoinIndexPipelineTestReconciler) parseResponseFile() string {
	filename := "index-empty"
	if x.ApiCurioResponseFilename != "" {
		filename = x.ApiCurioResponseFilename
	}
	responseTemplate, err := os.ReadFile("./test/data/apicurio/" + filename + ".json")
	checkError(err)
	tmpl, err := template.New("apicurioResponse").Parse(string(responseTemplate))
	checkError(err)
	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, map[string]interface{}{"Namespace": "xjoinindexpipeline." + x.Name})
	checkError(err)
	return templateBuffer.String()
}

func (x *XJoinIndexPipelineTestReconciler) registerValidMocks() {
	x.registerUpdatedMocks(UpdatedMocksParams{
		GraphQLSchemaExistingState: "DISABLED",
		GraphQLSchemaNewState:      "ENABLED",
	})
}

func (x *XJoinIndexPipelineTestReconciler) parseElasticsearchIndex(filename string) string {
	if filename == "" {
		if x.ElasticsearchIndexFileName == "" {
			filename = "get-index-response"
		} else {
			filename = x.ElasticsearchIndexFileName
		}
	}

	indexResponseTemplate, err := os.ReadFile("./test/data/elasticsearch/" + filename + ".json")
	checkError(err)
	tmpl, err := template.New("esIndex").Parse(string(indexResponseTemplate))
	checkError(err)
	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, map[string]interface{}{"IndexName": "xjoinindexpipeline." + x.GetName()})
	checkError(err)
	return templateBuffer.String()
}

func (x *XJoinIndexPipelineTestReconciler) parseGraphQLSchemaLabels(name string, labelsOverride []string) (labels []string) {
	if labelsOverride != nil {
		labels = labelsOverride
	} else {
		labels = []string{
			fmt.Sprintf("xjoin-subgraph-url=http://%s:4000/graphql", strings.ReplaceAll(name, ".", "-")),
			"graphql",
		}
	}
	return
}

func (x *XJoinIndexPipelineTestReconciler) registerUpdatedMocks(params UpdatedMocksParams) {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//elasticsearch mocks
	httpmock.RegisterResponder(
		"HEAD",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(200, `{}`))

	esIndexResponse := x.parseElasticsearchIndex(params.ElasticsearchIndexFilename)
	httpmock.RegisterResponder(
		"GET",
		"http://localhost:9200/xjoinindexpipeline."+x.GetName(),
		httpmock.NewStringResponder(200, esIndexResponse))

	httpmock.RegisterResponder(
		"GET",
		"http://localhost:9200/_cat/indices/"+x.Name+".%2A",
		httpmock.NewStringResponder(200, "[]"))

	httpmock.RegisterResponder(
		"GET",
		"http://localhost:9200/_ingest/pipeline/"+x.Name+"%2A",
		httpmock.NewStringResponder(200, "{}"))

	//graphql schema state update mocks
	var err error
	var getMetaResponder httpmock.Responder

	if params.GraphQLSchemaExistingState == "ENABLED" {
		getMetaResponder, err = httpmock.NewJsonResponder(200, map[string]interface{}{
			"state":  "ENABLED",
			"labels": x.parseGraphQLSchemaLabels(x.GetName(), params.GraphQLSchemaLabels)})
		checkError(err)
	} else {
		getMetaResponder, err = httpmock.NewJsonResponder(200, map[string]interface{}{
			"state":  "DISABLED",
			"labels": x.parseGraphQLSchemaLabels(x.GetName(), params.GraphQLSchemaLabels)})
		checkError(err)
	}

	var matcher httpmock.Matcher
	if params.GraphQLSchemaNewState == "ENABLED" {
		matcher = httpmock.BodyContainsString(`"state":"ENABLED"`).WithName("EnabledState")
	} else {
		matcher = httpmock.BodyContainsString(`"state":"DISABLED"`).WithName("DisabledState")
	}

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/meta",
		getMetaResponder)

	httpmock.RegisterMatcherResponder(
		"PUT",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/state",
		matcher,
		httpmock.NewStringResponder(200, `{}`))

	//avro schema mocks
	response := x.parseResponseFile()
	checkError(err)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(200, response))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, response))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, response))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects",
		httpmock.NewStringResponder(200, `[]`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+x.GetName()+"/versions",
		httpmock.NewStringResponder(200, `{}`))

	responder, err := httpmock.NewJsonResponder(200, httpmock.File("./test/data/apicurio/empty-response.json"))
	checkError(err)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/search/artifacts?limit=500&labels=graphql",
		responder)

	for _, customImage := range x.CustomSubgraphImages {
		var customGetMetaResponder httpmock.Responder

		customImageFullName := x.Name + "-" + customImage.Name + "." + x.Version

		if params.GraphQLSchemaExistingState == "ENABLED" {
			customGetMetaResponder, err = httpmock.NewJsonResponder(200, map[string]interface{}{
				"state":  "ENABLED",
				"labels": x.parseGraphQLSchemaLabels(customImageFullName, params.GraphQLSchemaLabels)})
			checkError(err)
		} else {
			customGetMetaResponder, err = httpmock.NewJsonResponder(200, map[string]interface{}{
				"state":  "DISABLED",
				"labels": x.parseGraphQLSchemaLabels(customImageFullName, params.GraphQLSchemaLabels)})
			checkError(err)
		}

		//custom subgraph graphql schema mocks
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/versions",
			customGetMetaResponder)

		httpmock.RegisterResponder(
			"POST",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts",
			httpmock.NewStringResponder(201, `{}`))

		httpmock.RegisterResponder(
			"PUT",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/meta",
			httpmock.NewStringResponder(200, `{}`))

		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/meta",
			customGetMetaResponder)

		httpmock.RegisterMatcherResponder(
			"PUT",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline."+customImageFullName+"/state",
			matcher,
			httpmock.NewStringResponder(200, `{}`))
	}

	for _, dataSource := range x.DataSources {
		response, err := os.ReadFile("./test/data/apicurio/" + dataSource.ApiCurioResponseFilename + ".json")
		checkError(err)
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+dataSource.Name+"."+dataSource.Version+"-value/versions/latest",
			httpmock.NewStringResponder(200, string(response)))

		httpmock.RegisterResponder(
			"GET",
			"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline."+x.GetName(),
			httpmock.NewStringResponder(404, "{}").Once())

		httpmock.RegisterResponder(
			"PUT",
			"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline."+x.GetName(),
			httpmock.NewStringResponder(200, "{}"))
	}

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, response))
}

func (x *XJoinIndexPipelineTestReconciler) newXJoinIndexPipelineReconciler() *controllers.XJoinIndexPipelineReconciler {
	return controllers.NewXJoinIndexPipelineReconciler(
		x.K8sClient,
		scheme.Scheme,
		testLogger,
		record.NewFakeRecorder(10),
		x.Namespace,
		true)
}

func (x *XJoinIndexPipelineTestReconciler) createValidIndexPipeline() {
	ctx := context.Background()
	indexAvroSchema, err := os.ReadFile("./test/data/avro/" + x.AvroSchemaFileName + ".json")
	checkError(err)
	xjoinIndexName := "test-xjoin-index"

	//XjoinIndexPipeline requires an XJoinIndex owner. Create one here
	indexSpec := v1alpha1.XJoinIndexSpec{
		AvroSchema:           string(indexAvroSchema),
		Pause:                false,
		CustomSubgraphImages: x.CustomSubgraphImages,
	}

	index := &v1alpha1.XJoinIndex{
		ObjectMeta: metav1.ObjectMeta{
			Name:      xjoinIndexName,
			Namespace: x.Namespace,
		},
		Spec: indexSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinIndex",
		},
	}

	Expect(x.K8sClient.Create(ctx, index)).Should(Succeed())

	//create the XJoinIndexPipeline
	indexPipelineSpec := v1alpha1.XJoinIndexPipelineSpec{
		Name:                 x.Name,
		Version:              x.Version,
		AvroSchema:           string(indexAvroSchema),
		Pause:                false,
		CustomSubgraphImages: x.CustomSubgraphImages,
	}

	blockOwnerDeletion := true
	controller := true
	indexPipeline := &v1alpha1.XJoinIndexPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      x.GetName(),
			Namespace: x.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         common.IndexGVK.Version,
					Kind:               common.IndexGVK.Kind,
					Name:               xjoinIndexName,
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
					UID:                "a6778b9b-dfed-4d41-af53-5ebbcddb7535",
				},
			},
		},
		Spec: indexPipelineSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinIndexPipeline",
		},
	}

	Expect(x.K8sClient.Create(ctx, indexPipeline)).Should(Succeed())

	//validate indexPipeline spec is created correctly
	indexPipelineLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	createdIndexPipeline := &v1alpha1.XJoinIndexPipeline{}

	Eventually(func() bool {
		err := x.K8sClient.Get(ctx, indexPipelineLookupKey, createdIndexPipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	Expect(createdIndexPipeline.Spec.Name).Should(Equal(x.Name))
	Expect(createdIndexPipeline.Spec.Version).Should(Equal(x.Version))
	Expect(createdIndexPipeline.Spec.Pause).Should(Equal(false))
	Expect(createdIndexPipeline.Spec.AvroSchema).Should(Equal(string(indexAvroSchema)))
	Expect(createdIndexPipeline.Spec.CustomSubgraphImages).Should(Equal(x.CustomSubgraphImages))
}
