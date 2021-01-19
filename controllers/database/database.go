package database

import (
	"fmt"
	"github.com/jackc/pgx"
	"strings"
)

type Database struct {
	connection *pgx.Conn
	Config     DBParams
}

type DBParams struct {
	User     string
	Password string
	Host     string
	Name     string
	Port     string
}

func NewDatabase(config DBParams) *Database {
	return &Database{
		Config: config,
	}
}

func (db *Database) Connect() (err error) {
	if db.connection, err = db.GetConnection(); err != nil {
		return fmt.Errorf("Error connecting to %s:%s/%s as %s : %s", db.Config.Host, db.Config.Port, db.Config.Name, db.Config.User, err)
	}

	return nil
}

func (db *Database) Close() error {
	if db.connection != nil {
		return db.connection.Close()
	}

	return nil
}

func (db *Database) RunQuery(query string) (*pgx.Rows, error) {
	rows, err := db.connection.Query(query)

	if err != nil {
		return nil, fmt.Errorf("Error executing query %s, %w", query, err)
	}

	return rows, nil
}

func (db *Database) Exec(query string) (result pgx.CommandTag, err error) {
	result, err = db.connection.Exec(query)

	if err != nil {
		return result, fmt.Errorf("Error executing query %s, %w", query, err)
	}

	return result, nil
}

func (db *Database) hostCountQuery() string {
	return fmt.Sprintf(`SELECT count(*) FROM hosts`)
}

func ReplicationSlotName(resourceNamePrefix string, pipelineVersion string) string {
	return strings.ReplaceAll(resourceNamePrefix, ".", "_") + "_" + pipelineVersion
}

func (db *Database) CreateReplicationSlot(slot string) error {
	rows, err := db.RunQuery(fmt.Sprintf("SELECT pg_create_physical_replication_slot('%s')", slot))
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

func (db *Database) ListReplicationSlots(resourceNamePrefix string) ([]string, error) {
	rows, err := db.RunQuery("SELECT slot_name from pg_catalog.pg_replication_slots")
	if err != nil {
		return nil, err
	}

	var slots []string

	defer rows.Close()
	for rows.Next() {
		var slot string
		err = rows.Scan(&slot)
		if err != nil {
			return slots, err
		}
		if strings.Index(slot, resourceNamePrefix) == 0 {
			slots = append(slots, slot)
		}
	}
	return slots, err
}

func (db *Database) RemoveReplicationSlot(slot string) error {
	rows, err := db.RunQuery(fmt.Sprintf(`SELECT pg_drop_replication_slot('%s')`, slot))
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

func (db *Database) RemoveReplicationSlotsForPipelineVersion(pipelineVersion string) error {
	rows, err := db.RunQuery("SELECT slot_name from pg_catalog.pg_replication_slots")
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
		if strings.Index(slot, pipelineVersion) != -1 {
			slots = append(slots, slot)
		}
	}
	rows.Close()

	for _, slot := range slots {
		dropRows, err := db.RunQuery(fmt.Sprintf(`SELECT pg_drop_replication_slot('%s')`, slot))
		if err != nil {
			return err
		}
		dropRows.Close()
	}

	return nil
}

func (db *Database) CountHosts() (int, error) {
	// TODO: add modified_on filter
	// waiting on https://issues.redhat.com/browse/RHCLOUD-9545
	rows, err := db.RunQuery(db.hostCountQuery())

	if err != nil {
		return -1, err
	}

	defer rows.Close()

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

	var ids []string

	if err != nil {
		return ids, err
	}

	defer rows.Close()

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

func (db *Database) GetConnection() (connection *pgx.Conn, err error) {
	const connectionStringTemplate = "host=%s user=%s password=%s port=%s"

	connStr := fmt.Sprintf(
		connectionStringTemplate,
		db.Config.Host,
		db.Config.User,
		db.Config.Password,
		db.Config.Port)

	//db.Config.Name is empty before creating the test database
	if db.Config.Name != "" {
		connStr = connStr + " dbname=" + db.Config.Name
	}

	if dbConfig, err := pgx.ParseDSN(connStr); err != nil {
		return nil, err
	} else {
		if connection, err = pgx.Connect(dbConfig); err != nil {
			return nil, err
		} else {
			return connection, nil
		}
	}
}

//used for tests
func (db *Database) CreateDatabase(name string) error {
	rows, err := db.RunQuery(fmt.Sprintf("CREATE DATABASE %s", name))
	if rows != nil {
		rows.Close()
	}
	return err
}

func (db *Database) DropDatabase(name string) error {
	rows, err := db.RunQuery(fmt.Sprintf("DROP DATABASE IF EXISTS %s", name))
	if rows != nil {
		rows.Close()
	}
	return err
}
