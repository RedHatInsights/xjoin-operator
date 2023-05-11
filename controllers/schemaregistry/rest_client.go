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
}

type Request struct {
	Method  string
	Path    string
	Body    string
	Headers map[string]string
}

func NewSchemaRegistryRestClient(connectionParams ConnectionParams) *RestClient {
	client := &http.Client{Timeout: time.Second * 60}
	return &RestClient{
		BaseUrl:    fmt.Sprintf("%s://%s:%s", connectionParams.Protocol, connectionParams.Hostname, connectionParams.Port) + "/apis/registry/v2",
		HttpClient: client,
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

	//add labels
	url := "http://" + strings.ReplaceAll(name, ".", "-") + ".test.svc:4000/graphql" //TODO url is static
	labelsBody := make(map[string]interface{})
	labelsBody["labels"] = []string{"xjoin-subgraph-url=" + url, "graphql"}
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
