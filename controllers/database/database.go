package database

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redhatinsights/xjoin-operator/controllers/data"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"strings"
	"text/template"
)

var log = logger.NewLogger("database")

type Database struct {
	connection *sqlx.DB
	Config     DBParams
}

type DBParams struct {
	User     string
	Password string
	Host     string
	Name     string
	Port     string
	SSL      string
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
	const connectionStringTemplate = "host=%s user=%s password=%s port=%s sslmode=%s"

	connStr := fmt.Sprintf(
		connectionStringTemplate,
		db.Config.Host,
		db.Config.User,
		db.Config.Password,
		db.Config.Port,
		db.Config.SSL)

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
	rows, err := db.connection.Queryx(query)

	if err != nil {
		return nil, fmt.Errorf("error executing query (%s) : %w", query, err)
	}

	return rows, nil
}

func (db *Database) ExecQuery(query string) (result sql.Result, err error) {
	result, err = db.connection.Exec(query)

	if err != nil {
		return result, fmt.Errorf("error executing query (%s) : %w", query, err)
	}

	return result, nil
}

func (db *Database) hostCountQuery() string {
	return fmt.Sprintf(`SELECT count(*) FROM hosts`)
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

	_, err := db.ExecQuery(fmt.Sprintf(
		`SELECT pg_drop_replication_slot('%s') WHERE EXISTS 
				 (SELECT slot_name from pg_catalog.pg_replication_slots where slot_name='%s')`, slot, slot))
	if err != nil {
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
	// TODO: add modified_on filter
	// waiting on https://issues.redhat.com/browse/RHCLOUD-9545
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

func (db *Database) hostIdQuery() string {
	return fmt.Sprintf(`SELECT id FROM hosts ORDER BY id`)
}

func (db *Database) GetHostIds() ([]string, error) {
	// TODO: add modified_on filter
	// waiting on https://issues.redhat.com/browse/RHCLOUD-9545
	rows, err := db.RunQuery(db.hostIdQuery())
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

func closeRows(rows *sqlx.Rows) {
	if rows != nil {
		err := rows.Close()
		if err != nil {
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

//TODO handle this dynamically with a schema
func (db *Database) GetHostsByIds(ids []string) ([]data.Host, error) {
	cols := "id,account,display_name,created_on,modified_on,facts,canonical_facts,system_profile_facts,ansible_host,stale_timestamp,reporter"

	idsMap := make(map[string]interface{})
	idsMap["IDs"] = ids

	tmpl, err := template.New("host-ids").Parse(`{{range $idx, $id := .IDs}}'{{$id}}',{{end}}`)
	if err != nil {
		return nil, err
	}

	var idsTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&idsTemplateBuffer, idsMap)
	if err != nil {
		return nil, err
	}
	idsTemplateParsed := idsTemplateBuffer.String()
	idsTemplateParsed = idsTemplateParsed[0 : len(idsTemplateParsed)-1]

	query := fmt.Sprintf("SELECT %s FROM hosts WHERE ID IN (%s) ORDER BY id", cols, idsTemplateParsed)

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

		systemProfileJson, err := parseJsonField(host.SystemProfileFacts.([]uint8))
		if err != nil {
			return nil, err
		}
		host.SystemProfileFacts = systemProfileJson

		canonicalFactsJson, err := parseJsonField(host.CanonicalFacts.([]uint8))
		if err != nil {
			return nil, err
		}
		host.CanonicalFacts = canonicalFactsJson

		factsJson, err := parseJsonField(host.Facts.([]uint8))
		if err != nil {
			return nil, err
		}
		host.Facts = factsJson

		response = append(response, host)
	}

	return response, nil
}
