package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"sync"

	"go.dedis.ch/cothority/v3/bypros/storage"
	"go.dedis.ch/cothority/v3/skipchain"
	"golang.org/x/xerrors"
)

// Registry is used by the test to cleanly stop all db connections.
var Registry ConnRegistry = ConnRegistry{}

// ConnRegistry is a global registry of opened DB connections. We use it in
// tests to be sure every connections are closed.
type ConnRegistry struct {
	sync.Mutex
	conns []*sql.DB
}

func (r *ConnRegistry) add(conn *sql.DB) {
	r.Lock()
	defer r.Unlock()

	r.conns = append(r.conns, conn)
}

// CloseAll closes all the connections registers in the registry.
func (r *ConnRegistry) CloseAll() {
	r.Lock()
	defer r.Unlock()
	for _, db := range r.conns {
		db.Close()
	}
}

// NewSQL returns a new SQL storage.
func NewSQL() (storage.Storage, error) {
	dbURL := os.Getenv("PROXY_DB_URL")
	if dbURL == "" {
		return nil, xerrors.Errorf("failed to get DB URL: did you set PROXY_DB_URL ?")
	}

	dbURLRO := os.Getenv("PROXY_DB_URL_RO")
	if dbURLRO == "" {
		return nil, xerrors.Errorf("failed to get DB URL: did you set PROXY_DB_URL_RO ?")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, xerrors.Errorf("failed to open db: %v", err)
	}

	Registry.add(db)

	dbRo, err := sql.Open("pgx", dbURLRO)
	if err != nil {
		return nil, xerrors.Errorf("failed to open dbRo: %v", err)
	}

	Registry.add(dbRo)

	return SQL{
		db:   db,
		dbRo: dbRo,
	}, nil
}

// SQL defines a storage based on an SQL database.
//
// - implements storage.Storage
type SQL struct {
	db   *sql.DB
	dbRo *sql.DB
}

// Close closes the connections
func (s SQL) Close() {
	s.db.Close()
	s.dbRo.Close()
}

// GetBlock implements storage.Storage. It returns -1 if the block is not found,
// otherwise its primary key.
func (s SQL) GetBlock(blockHash []byte) (int, error) {
	var blockID int

	query := `SELECT block_id
	FROM cothority.block
	WHERE hash=$1`

	err := s.dbRo.QueryRow(query, blockHash).Scan(&blockID)

	if err == nil {
		return blockID, nil
	}
	if err != sql.ErrNoRows {
		return -1, xerrors.Errorf("failed to fetch block: %v", err)
	}

	return -1, nil
}

// StoreBlock implements storage.Storage. It stores the block and returns its
// primary key.
func (s SQL) StoreBlock(block *skipchain.SkipBlock) (int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sqlTx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return -1, xerrors.Errorf("failed to begin transaction: %v", err)
	}

	storeTx := storeTx{
		sqlTx: sqlTx,
		block: block,
	}

	blockID, err := storeTx.store()
	if err != nil {
		return 1, xerrors.Errorf("failed to store block: %v", err)
	}

	err = sqlTx.Commit()
	if err != nil {
		return -1, xerrors.Errorf("failed to commit transaction: %v", err)
	}

	return blockID, err
}

// Query implements storage.Storage. It uses a read-only user to perform the
// query and returns back a JSON representation of the result.
func (s SQL) Query(query string) ([]byte, error) {
	rows, err := s.dbRo.Query(query)
	if err != nil {
		return nil, xerrors.Errorf("failed to execute query: %v", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, xerrors.Errorf("failed to get column types: %v", err)
	}

	json, err := rowsToJSON(columnTypes, rows)
	if err != nil {
		return nil, xerrors.Errorf("failed to convert to JSON: %v", err)
	}

	return json, nil
}

// rowsToJSON converts a database result to JSON.
func rowsToJSON(columnTypes []*sql.ColumnType, rows *sql.Rows) ([]byte, error) {
	count := len(columnTypes)
	finalRows := []interface{}{}

	// column parsing is taken from https://stackoverflow.com/a/52572145
	for rows.Next() {

		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {
			switch v.DatabaseTypeName() {
			case "VARCHAR", "TEXT", "UUID", "TIMESTAMP":
				scanArgs[i] = new(sql.NullString)
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
			case "INT4":
				scanArgs[i] = new(sql.NullInt64)
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)
		if err != nil {
			return nil, xerrors.Errorf("failed to scan: %v", err)
		}

		masterData := parseColumn(columnTypes, scanArgs)
		finalRows = append(finalRows, masterData)
	}

	z, err := json.MarshalIndent(finalRows, "", "  ")
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal JSON: %v", err)
	}

	return z, nil
}

func parseColumn(columnTypes []*sql.ColumnType, scanArgs []interface{}) map[string]interface{} {

	masterData := map[string]interface{}{}

	for i, v := range columnTypes {

		if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
			masterData[v.Name()] = z.Bool
			continue
		}

		if z, ok := (scanArgs[i]).(*sql.NullString); ok {
			masterData[v.Name()] = z.String
			continue
		}

		if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
			masterData[v.Name()] = z.Int64
			continue
		}

		if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
			masterData[v.Name()] = z.Float64
			continue
		}

		if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
			masterData[v.Name()] = z.Int32
			continue
		}

		masterData[v.Name()] = scanArgs[i]
	}

	return masterData
}
