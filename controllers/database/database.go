package database

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"

	"github.com/go-errors/errors"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redhatinsights/xjoin-operator/controllers/data"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
)

var log = logger.NewLogger("database")

type Database struct {
	connection *sqlx.DB
	Config     DBParams
}

type DBParams struct {
	User        string
	Password    string
	Host        string
	Name        string
	Port        string
	SSLMode     string
	SSLRootCert string
}

func NewDatabase(config DBParams) *Database {
	return &Database{
		Config: config,
	}
}

func (db *Database) Connect() (err error) {
	if db.connection != nil {
		return nil
	}

	if db.connection, err = db.GetConnection(); err != nil {
		return fmt.Errorf("error connecting to %s:%s/%s as %s : %s", db.Config.Host, db.Config.Port, db.Config.Name, db.Config.User, err)
	}

	return nil
}

func (db *Database) GetConnection() (connection *sqlx.DB, err error) {
	connectionStringTemplate := "host=%s user=%s password=%s port=%s sslmode=%s"

	if db.Config.SSLMode != "disable" {
		connectionStringTemplate = connectionStringTemplate + " sslrootcert=" + db.Config.SSLRootCert
	}

	connStr := fmt.Sprintf(
		connectionStringTemplate,
		db.Config.Host,
		db.Config.User,
		db.Config.Password,
		db.Config.Port,
		db.Config.SSLMode)

	//db.Config.Name is empty before creating the test database
	if db.Config.Name != "" {
		connStr = connStr + " dbname=" + db.Config.Name
	}

	if connection, err = sqlx.Connect("postgres", connStr); err != nil {
		return nil, err
	} else {
		return connection, nil
	}
}

func (db *Database) Close() error {
	if db.connection != nil {
		return db.connection.Close()
	}

	return nil
}

func (db *Database) SetMaxConnections(numConnections int) {
	if db.connection != nil {
		db.connection.DB.SetMaxOpenConns(numConnections)
		db.connection.DB.SetMaxIdleConns(numConnections)
	}
}

func (db *Database) RunQuery(query string) (*sqlx.Rows, error) {
	if db.connection == nil {
		return nil, errors.New("cannot run query because there is no database connection")
	}
	rows, err := db.connection.Queryx(query)

	if err != nil {
		return nil, fmt.Errorf("error executing query (%s) : %w", query, err)
	}

	return rows, nil
}

func (db *Database) ExecQuery(query string) (result sql.Result, err error) {
	if db.connection == nil {
		return nil, errors.New("cannot run query because there is no database connection")
	}
	result, err = db.connection.Exec(query)

	if err != nil {
		return result, fmt.Errorf("error executing query (%s) : %w", query, err)
	}

	return result, nil
}

func (db *Database) hostCountQuery() string {
	return "SELECT count(*) FROM hosts"
}

func ReplicationSlotName(resourceNamePrefix string, pipelineVersion string) string {
	return ReplicationSlotPrefix(resourceNamePrefix) + "_" + pipelineVersion
}

func ReplicationSlotPrefix(resourceNamePrefix string) string {
	return strings.ReplaceAll(resourceNamePrefix, ".", "_")
}

func (db *Database) CreateReplicationSlot(slot string) error {
	_, err := db.ExecQuery(fmt.Sprintf("SELECT pg_create_physical_replication_slot('%s')", slot))
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) ListReplicationSlots(resourceNamePrefix string) ([]string, error) {
	rows, err := db.RunQuery("SELECT slot_name from pg_catalog.pg_replication_slots")
	defer closeRows(rows)
	if err != nil {
		return nil, err
	}

	var slots []string

	for rows.Next() {
		var slot string
		err = rows.Scan(&slot)
		if err != nil {
			return slots, err
		}
		if strings.Index(slot, ReplicationSlotPrefix(resourceNamePrefix)) == 0 {
			slots = append(slots, slot)
		}
	}
	return slots, err
}

func (db *Database) RemoveReplicationSlot(slot string) error {
	if slot == "" {
		return nil
	}

	totalAttempts := 10
	success := false
	var err error

	for attempt := 0; attempt < totalAttempts; attempt++ {
		_, err = db.ExecQuery(fmt.Sprintf(
			`SELECT pg_drop_replication_slot('%s') WHERE EXISTS 
				 (SELECT slot_name from pg_catalog.pg_replication_slots where slot_name='%s')`, slot, slot))
		if err == nil {
			success = true
			break
		} else {
			time.Sleep(time.Second * 1)
		}
	}

	if !success {
		return err
	}

	return nil
}

func (db *Database) RemoveReplicationSlotsForPrefix(resourceNamePrefix string) error {
	prefix := ReplicationSlotPrefix(resourceNamePrefix)
	rows, err := db.RunQuery(
		fmt.Sprintf("SELECT slot_name from pg_catalog.pg_replication_slots WHERE slot_name LIKE '%s%%'", prefix))
	defer closeRows(rows)
	if err != nil {
		return err
	}

	var slots []string
	for rows.Next() {
		var slot string
		err = rows.Scan(&slot)
		if err != nil {
			return err
		}
		slots = append(slots, slot)
	}

	for _, slot := range slots {
		_, err = db.ExecQuery(fmt.Sprintf(`SELECT pg_drop_replication_slot('%s')`, slot))
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) RemoveReplicationSlotsForPipelineVersion(pipelineVersion string) error {
	rows, err := db.RunQuery(
		fmt.Sprintf("SELECT slot_name from pg_catalog.pg_replication_slots WHERE slot_name LIKE '%%%s%%'", pipelineVersion))
	defer closeRows(rows)
	if err != nil {
		return err
	}

	var slots []string
	for rows.Next() {
		var slot string
		err = rows.Scan(&slot)
		if err != nil {
			return err
		}
		slots = append(slots, slot)
	}

	for _, slot := range slots {
		_, err := db.ExecQuery(fmt.Sprintf(`SELECT pg_drop_replication_slot('%s')`, slot))
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) CountHosts() (int, error) {
	rows, err := db.RunQuery(db.hostCountQuery())
	defer closeRows(rows)

	if err != nil {
		return -1, err
	}

	var response int
	for rows.Next() {
		var count int
		err = rows.Scan(&count)
		if err != nil {
			return -1, err
		}
		response = count
	}

	return response, err
}

func (db *Database) QueryIds(query string) ([]string, error) {
	rows, err := db.RunQuery(query)
	defer closeRows(rows)

	var ids []string

	if err != nil {
		return ids, err
	}

	for rows.Next() {
		var id string
		err = rows.Scan(&id)

		if err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (db *Database) GetHostIdsByIdList(ids []string) ([]string, error) {
	log.Debug("Retrieving ids from DB: ", "ids list (max 50)", ids[:utils.Min(50, len(ids))], "total", len(ids))
	idsString, err := formatIdsList(ids)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`SELECT id FROM hosts WHERE id in (%s)`, idsString)
	return db.QueryIds(query)
}

func (db *Database) GetHostIdsByModifiedOn(start time.Time, end time.Time) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT id FROM hosts WHERE modified_on > '%s' AND modified_on < '%s' ORDER BY id `,
		start.Format(utils.TimeFormat()), end.Format(utils.TimeFormat()))

	log.Info("GetHostIdsQuery", "query", query)

	return db.QueryIds(query)
}

func closeRows(rows *sqlx.Rows) {
	if rows != nil {
		err := rows.Close()
		if err != nil {
			err = errors.Wrap(err, 0)
			log.Error(err, "Unable to close rows")
		}
	}
}

func parseJsonField(field []uint8) (map[string]interface{}, error) {
	fieldMap := make(map[string]interface{})
	err := json.Unmarshal(field, &fieldMap)
	if err != nil {
		return nil, err
	}
	return fieldMap, nil
}

func formatIdsList(ids []string) (string, error) {
	idsMap := make(map[string]interface{})
	idsMap["IDs"] = ids

	tmpl, err := template.New("host-ids").Parse(`{{range $idx, $id := .IDs}}'{{$id}}',{{end}}`)
	if err != nil {
		return "", err
	}

	var idsTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&idsTemplateBuffer, idsMap)
	if err != nil {
		return "", err
	}
	idsTemplateParsed := idsTemplateBuffer.String()
	idsTemplateParsed = idsTemplateParsed[0 : len(idsTemplateParsed)-1]
	return idsTemplateParsed, nil
}

// TODO handle this dynamically with a schema
func (db *Database) GetHostsByIds(ids []string) ([]data.Host, error) {
	cols := "id,account,org_id,display_name,created_on,modified_on,facts,canonical_facts,system_profile_facts,ansible_host,stale_timestamp,reporter,tags"

	idsString, err := formatIdsList(ids)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		"SELECT %s FROM hosts WHERE ID IN (%s) ORDER BY id",
		cols, idsString)

	rows, err := db.connection.Queryx(query)
	defer closeRows(rows)

	if err != nil {
		return nil, fmt.Errorf("error executing query %s, %w", query, err)
	}

	var response []data.Host

	for rows.Next() {
		var host data.Host
		err = rows.StructScan(&host)

		if err != nil {
			return nil, err
		}

		if host.SystemProfileFacts != nil {
			systemProfileJson, err := parseJsonField(host.SystemProfileFacts.([]uint8))
			if err != nil {
				return nil, err
			}
			host.SystemProfileFacts = systemProfileJson
		}

		if host.CanonicalFacts != nil {
			canonicalFactsJson, err := parseJsonField(host.CanonicalFacts.([]uint8))
			if err != nil {
				return nil, err
			}
			host.CanonicalFacts = canonicalFactsJson
		}

		if host.Facts != nil {
			factsJson, err := parseJsonField(host.Facts.([]uint8))
			if err != nil {
				return nil, err
			}
			host.Facts = factsJson
		}

		if host.Tags != nil {
			tagsJson, err := parseJsonField(host.Tags.([]uint8))
			if err != nil {
				return nil, err
			}
			host.Tags = tagsJson
			host.TagsStructured, host.TagsString, host.TagsSearch = tagsStructured(tagsJson)
		}

		response = append(response, host)
	}

	return response, nil
}

/*
"tags_structured": [

	{
		"namespace": "NS1",
		"value": "val3",
		"key": "key3"
	}

],

"tags_string": [

	"NS1/key3/val3",
	"NS3/key3/val3",
	"Sat/prod/",
	"SPECIAL/key/val"

],

"tags_search": [

	"NS1/key3=val3",
	"NS3/key3=val3",
	"Sat/prod=",
	"SPECIAL/key=val"

],
*/
func tagsStructured(tagsJson map[string]interface{}) (
	structuredTags []map[string]string,
	stringsTags []string,
	searchTags []string) {

	structuredTags = make([]map[string]string, 0)
	stringsTags = make([]string, 0)
	searchTags = make([]string, 0)

	tagsJson = utils.SortMap(tagsJson)

	for namespaceName, namespaceVal := range tagsJson {
		namespaceMap := utils.SortMap(namespaceVal.(map[string]interface{}))
		for keyName, values := range namespaceMap {
			valuesArray := values.([]interface{})

			if len(valuesArray) == 0 {
				valuesArray = append(valuesArray, "")
			}

			for _, val := range valuesArray {
				structuredTag := make(map[string]string)
				structuredTag["key"] = keyName
				structuredTag["namespace"] = namespaceName
				structuredTag["value"] = val.(string)
				structuredTags = append(structuredTags, structuredTag)

				stringTag := url.QueryEscape(namespaceName)
				stringTag = stringTag + "/" + url.QueryEscape(keyName)
				stringTag = stringTag + "/" + url.QueryEscape(val.(string))
				stringsTags = append(stringsTags, stringTag)

				if namespaceName == "null" {
					namespaceName = ""
				}

				searchTag := namespaceName
				searchTag = searchTag + "/" + keyName
				searchTag = searchTag + "=" + val.(string)
				searchTags = append(searchTags, searchTag)
			}
		}
	}

	data.OrderedBy(data.NamespaceComparator, data.KeyComparator, data.ValueComparator).Sort(structuredTags)
	sort.Strings(stringsTags)
	sort.Strings(searchTags)

	return structuredTags, stringsTags, searchTags
}
