package schemaregistry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type RestClient struct {
	BaseUrl    string
	HttpClient *http.Client
	Namespace  string
}

type Request struct {
	Method  string
	Path    string
	Body    string
	Headers map[string]string
}

func NewSchemaRegistryRestClient(connectionParams ConnectionParams, namespace string) *RestClient {
	client := &http.Client{Timeout: time.Second * 60}
	return &RestClient{
		BaseUrl:    fmt.Sprintf("%s://%s:%s", connectionParams.Protocol, connectionParams.Hostname, connectionParams.Port) + "/apis/registry/v2",
		HttpClient: client,
		Namespace:  namespace,
	}
}

func (c *RestClient) MakeRequest(requestParams Request) (resCode int, body map[string]interface{}, err error) {
	req, err := http.NewRequest(requestParams.Method, c.BaseUrl+requestParams.Path, bytes.NewReader([]byte(requestParams.Body)))
	if err != nil {
		return 500, nil, errors.Wrap(err, 0)
	}

	for headerName, headerValue := range requestParams.Headers {
		req.Header.Add(headerName, headerValue)
	}
	res, err := c.HttpClient.Do(req)
	if err != nil {
		return 500, nil, errors.Wrap(err, 0)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return 500, nil, errors.Wrap(err, 0)
	}

	if strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		err = json.Unmarshal(resBody, &body)
		if err != nil {
			return 500, nil, errors.Wrap(err, 0)
		}

		err = res.Body.Close()
		if err != nil {
			return 500, nil, errors.Wrap(err, 0)
		}
	}

	return res.StatusCode, body, nil
}

func (c *RestClient) BuildGraphQLSchemaLabels(name string) []interface{} {
	name = strings.ReplaceAll(name, "xjoinindexpipeline.", "")
	url := "http://" + strings.ReplaceAll(name, ".", "-") + ":4000/graphql"
	labels := []interface{}{"xjoin-subgraph-url=" + url, "graphql"}
	return labels
}

func (c *RestClient) RegisterGraphQLSchema(name string) (id string, err error) {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/graphql"
	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodPost,
		Path:   "/groups/default/artifacts",
		Body:   "type Query {internalServerError: string}",
		Headers: map[string]string{
			"Content-Type":            "application/graphql",
			"X-Registry-ArtifactId":   name,
			"X-Registry-ArtifactType": "GRAPHQL",
		},
	})

	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		return "", errors.Wrap(errors.New(fmt.Sprintf(
			"unable to create graphql schema, statusCode: %v, message: %s", resCode, resBody["message"])), 0)
	}

	//set schema state to disabled
	//it will be enabled when the pipeline becomes valid
	err = c.DisableSchema(name)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	//add labels
	labelsBody := make(map[string]interface{})
	labelsBody["labels"] = c.BuildGraphQLSchemaLabels(name)
	labelsBodyJson, err := json.Marshal(labelsBody)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	resCode, resBody, err = c.MakeRequest(Request{
		Method: http.MethodPut,
		Path:   "/groups/default/artifacts/" + name + "/meta",
		Body:   string(labelsBodyJson),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		return "", errors.Wrap(errors.New(fmt.Sprintf(
			"unable to create graphql schema labels, statusCode: %v, message: %s", resCode, resBody["message"])), 0)
	}

	return name, nil
}

func (c *RestClient) GetSchemaState(name string) (state string, err error) {
	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodGet,
		Path:   "/groups/default/artifacts/" + name + "/meta",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	})
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		return "", errors.Wrap(errors.New(fmt.Sprintf(
			"unable to get schema state, statusCode: %v, message: %s", resCode, resBody["message"])), 0)
	}

	state, ok := resBody["state"].(string)
	if !ok {
		return "", errors.Wrap(errors.New(fmt.Sprintf(
			"invalid response when getting schema state: %s", resBody)), 0)
	}
	return state, nil
}

func (c *RestClient) IsSchemaEnabled(name string) (bool, error) {
	state, err := c.GetSchemaState(name)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if state == "ENABLED" {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *RestClient) SetSchemaState(name string, state string) (err error) {
	stateBody := make(map[string]interface{})
	stateBody["state"] = state
	stateBodyJson, err := json.Marshal(stateBody)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodPut,
		Path:   "/groups/default/artifacts/" + name + "/state",
		Body:   string(stateBodyJson),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		return errors.Wrap(errors.New(fmt.Sprintf(
			"unable to set schema state, statusCode: %v, message: %s", resCode, resBody["message"])), 0)
	}

	return nil
}

func (c *RestClient) EnableSchema(name string) (err error) {
	alreadyEnabled, err := c.IsSchemaEnabled(name)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if !alreadyEnabled {
		err = c.SetSchemaState(name, "ENABLED")
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}

func (c *RestClient) DisableSchema(name string) (err error) {
	isEnabled, err := c.IsSchemaEnabled(name)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if isEnabled {
		err = c.SetSchemaState(name, "DISABLED")
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}

func (c *RestClient) DeleteGraphQLSchema(name string) (err error) {
	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodDelete,
		Path:   "/groups/default/artifacts/" + name,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		return errors.Wrap(errors.New(fmt.Sprintf(
			"unable to delete graphql schema, schema: %s, statusCode: %v message: %s",
			name, resCode, resBody["message"])), 0)
	}
	return nil
}

func (c *RestClient) CheckIfGraphQLSchemaExists(name string) (exists bool, err error) {
	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodGet,
		Path:   "/groups/default/artifacts/" + name + "/versions",
	})
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if resCode >= 300 && resCode != 404 {
		return false, errors.Wrap(errors.New(fmt.Sprintf(
			"unable to check if graphql schema exists, statusCode: %v, message: %s",
			resCode, resBody["message"])), 0)
	} else if resCode == 404 {
		return false, nil
	} else {
		return true, nil
	}
}

func (c *RestClient) ListVersionsForSchemaName(schemaName string) (versions []string, err error) {
	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodGet,
		Path:   "/search/artifacts?limit=500&labels=graphql",
	})

	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		return nil, errors.Wrap(errors.New(fmt.Sprintf(
			"unable to list graphql schema names, statusCode %v, message %s",
			resCode, resBody["message"])), 0)
	}

	artifacts, ok := resBody["artifacts"].([]interface{})
	if !ok {
		return nil, errors.Wrap(errors.New(
			"Invalid response from apicurio GET /search/artifacts, artifacts is not an array"), 0)
	}

	for _, schema := range artifacts {
		schemaMap := schema.(map[string]interface{})
		schemaId := schemaMap["id"].(string)
		schemaIdParts := strings.Split(schemaId, ".")
		if len(schemaIdParts) != 3 {
			return nil, errors.Wrap("encountered invalid schema name", 0)
		}
		if strings.Index(schemaIdParts[1], schemaName) == 0 {
			version := strings.Split(schemaId, ".")[2]
			versions = append(versions, version)
		}
	}

	return versions, nil
}

func (c *RestClient) GetSchemaLabels(schemaName string) ([]interface{}, error) {
	resCode, resBody, err := c.MakeRequest(Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("/groups/default/artifacts/%s/meta", schemaName),
	})

	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if resCode >= 300 {
		message := ""
		if resBody != nil {
			message, _ = resBody["message"].(string)
		}
		return nil, errors.Wrap(errors.New(fmt.Sprintf(
			"unable to get schema metadata, statusCode %v, message %s",
			resCode, message)), 0)
	}

	response, ok := resBody["labels"].([]interface{})
	if !ok {
		return nil, errors.Wrap(errors.New(
			"metadata field 'labels' is not an array for schema: "+schemaName), 0)
	}
	return response, nil
}
