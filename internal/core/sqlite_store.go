package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	_ "modernc.org/sqlite"
)

const observationTimeLayout = "2006-01-02T15:04:05.000000000Z"

type SQLiteStore struct { db *sql.DB }

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" { return nil, errors.New("sqlite path is required") }
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return nil, err }
	db, err := sql.Open("sqlite", path); if err != nil { return nil, err }
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil { _=db.Close(); return nil,err }
	if err := store.normalizeLegacyTimes(); err != nil { _=db.Close(); return nil,err }
	return store,nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS observations (id INTEGER PRIMARY KEY AUTOINCREMENT, agent_id TEXT NOT NULL, observed_at TEXT NOT NULL, endpoint_host TEXT NOT NULL, endpoint_port TEXT, endpoint_tls INTEGER NOT NULL, payload_json BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS idx_observations_agent_time ON observations(agent_id, observed_at);
CREATE INDEX IF NOT EXISTS idx_observations_endpoint_time ON observations(endpoint_host, endpoint_port, observed_at);
CREATE TABLE IF NOT EXISTS incident_records (id INTEGER PRIMARY KEY AUTOINCREMENT, endpoint_host TEXT NOT NULL, endpoint_port TEXT, endpoint_tls INTEGER NOT NULL, started_at TEXT NOT NULL, status TEXT NOT NULL, payload_json BLOB NOT NULL, UNIQUE(endpoint_host, endpoint_port, endpoint_tls, started_at));
CREATE INDEX IF NOT EXISTS idx_incident_records_started ON incident_records(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_incident_records_status ON incident_records(status, started_at DESC);
CREATE TABLE IF NOT EXISTS network_incident_records (id INTEGER PRIMARY KEY AUTOINCREMENT, network_id TEXT NOT NULL, started_at TEXT NOT NULL, status TEXT NOT NULL, severity TEXT NOT NULL, payload_json BLOB NOT NULL, UNIQUE(network_id, started_at));
CREATE INDEX IF NOT EXISTS idx_network_incident_records_started ON network_incident_records(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_network_incident_records_status ON network_incident_records(status, started_at DESC);
CREATE TABLE IF NOT EXISTS networks (id TEXT PRIMARY KEY, name TEXT NOT NULL, website TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '');
CREATE UNIQUE INDEX IF NOT EXISTS idx_networks_name ON networks(name COLLATE NOCASE);
CREATE TABLE IF NOT EXISTS network_servers (id TEXT PRIMARY KEY, network_id TEXT NOT NULL, name TEXT NOT NULL, FOREIGN KEY(network_id) REFERENCES networks(id));
CREATE INDEX IF NOT EXISTS idx_network_servers_network ON network_servers(network_id, name);
CREATE TABLE IF NOT EXISTS network_endpoints (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, host TEXT NOT NULL, port TEXT NOT NULL, tls INTEGER NOT NULL, FOREIGN KEY(server_id) REFERENCES network_servers(id), UNIQUE(server_id, host, port, tls));
CREATE INDEX IF NOT EXISTS idx_network_endpoints_server ON network_endpoints(server_id, host, port);`)
	return err
}

func (s *SQLiteStore) normalizeLegacyTimes() error {
	if s == nil || s.db == nil { return errors.New("sqlite store is not open") }
	type timestampUpdate struct { id int64; value string }
	observationRows, err := s.db.Query(`SELECT id, payload_json FROM observations`); if err != nil{return err}
	observationUpdates:=make([]timestampUpdate,0)
	for observationRows.Next(){var id int64;var payload []byte;if err:=observationRows.Scan(&id,&payload);err!=nil{_ = observationRows.Close();return err};var observation agent.Observation;if err:=json.Unmarshal(payload,&observation);err!=nil{_ = observationRows.Close();return err};observationUpdates=append(observationUpdates,timestampUpdate{id,formatObservationTime(observation.ObservedAt)})}
	if err:=observationRows.Err();err!=nil{_ = observationRows.Close();return err};if err:=observationRows.Close();err!=nil{return err}
	incidentRows,err:=s.db.Query(`SELECT id, payload_json FROM incident_records`);if err!=nil{return err};incidentUpdates:=make([]timestampUpdate,0)
	for incidentRows.Next(){var id int64;var payload []byte;if err:=incidentRows.Scan(&id,&payload);err!=nil{_ = incidentRows.Close();return err};var incident IncidentLifecycle;if err:=json.Unmarshal(payload,&incident);err!=nil{_ = incidentRows.Close();return err};incidentUpdates=append(incidentUpdates,timestampUpdate{id,formatObservationTime(incident.StartedAt)})}
	if err:=incidentRows.Err();err!=nil{_ = incidentRows.Close();return err};if err:=incidentRows.Close();err!=nil{return err}
	tx,err:=s.db.Begin();if err!=nil{return err};defer tx.Rollback()
	for _,u:=range observationUpdates{if _,err:=tx.Exec(`UPDATE observations SET observed_at = ? WHERE id = ?`,u.value,u.id);err!=nil{return err}}
	for _,u:=range incidentUpdates{if _,err:=tx.Exec(`UPDATE incident_records SET started_at = ? WHERE id = ?`,u.value,u.id);err!=nil{return err}}
	return tx.Commit()
}

func (s *SQLiteStore) Store(observation agent.Observation) error {
	if s==nil||s.db==nil{return errors.New("sqlite store is not open")};payload,err:=json.Marshal(observation);if err!=nil{return err}
	_,err=s.db.Exec(`INSERT INTO observations (agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json) VALUES (?, ?, ?, ?, ?, ?)`,observation.AgentID,formatObservationTime(observation.ObservedAt),observation.Endpoint.Host,observation.Endpoint.Port,observation.Endpoint.TLS,payload);if err!=nil{return err}
	if err:=s.refreshIncidentRecords();err!=nil{return err};return s.refreshNetworkIncidentRecords()
}

func (s *SQLiteStore) List(query ObservationQuery) ([]agent.Observation,error){if s==nil||s.db==nil{return nil,errors.New("sqlite store is not open")};if query.Limit<1||query.Limit>maxObservationLimit{return nil,errors.New("invalid observation limit")};if !query.Since.IsZero()&&!query.Until.IsZero()&&query.Since.After(query.Until){return nil,errors.New("invalid observation time range")};where:=make([]string,0,4);args:=make([]any,0,5);if query.AgentID!=""{where=append(where,"agent_id = ?");args=append(args,query.AgentID)};if query.Host!=""{where=append(where,"endpoint_host = ?");args=append(args,query.Host)};if !query.Since.IsZero(){where=append(where,"observed_at >= ?");args=append(args,formatObservationTime(query.Since))};if !query.Until.IsZero(){where=append(where,"observed_at <= ?");args=append(args,formatObservationTime(query.Until))};statement:="SELECT payload_json FROM observations";if len(where)>0{statement+=" WHERE "+strings.Join(where," AND ")};statement+=" ORDER BY observed_at DESC, id DESC LIMIT ?";args=append(args,query.Limit);rows,err:=s.db.Query(statement,args...);if err!=nil{return nil,err};defer rows.Close();return decodeObservationRows(rows)}
func (s *SQLiteStore) LatestEndpointObservations()([]agent.Observation,error){if s==nil||s.db==nil{return nil,errors.New("sqlite store is not open")};rows,err:=s.db.Query(`SELECT payload_json FROM (SELECT payload_json, ROW_NUMBER() OVER (PARTITION BY agent_id, endpoint_host, endpoint_port, endpoint_tls ORDER BY observed_at DESC, id DESC) AS row_number FROM observations) WHERE row_number = 1`);if err!=nil{return nil,err};defer rows.Close();return decodeObservationRows(rows)}
func (s *SQLiteStore) RecentObservations(limit int)([]agent.Observation,error){if s==nil||s.db==nil{return nil,errors.New("sqlite store is not open")};if limit<1{return nil,errors.New("invalid recent observation limit")};rows,err:=s.db.Query(`SELECT payload_json FROM observations ORDER BY observed_at DESC, id DESC LIMIT ?`,limit);if err!=nil{return nil,err};defer rows.Close();return decodeObservationRows(rows)}
func decodeObservationRows(rows *sql.Rows)([]agent.Observation,error){out:=make([]agent.Observation,0);for rows.Next(){var payload []byte;if err:=rows.Scan(&payload);err!=nil{return nil,err};var o agent.Observation;if err:=json.Unmarshal(payload,&o);err!=nil{return nil,err};out=append(out,o)};if err:=rows.Err();err!=nil{return nil,err};return out,nil}
func formatObservationTime(value time.Time)string{return value.UTC().Format(observationTimeLayout)}
func (s *SQLiteStore) Count()(int,error){if s==nil||s.db==nil{return 0,errors.New("sqlite store is not open")};var count int;if err:=s.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&count);err!=nil{return 0,err};return count,nil}
func (s *SQLiteStore) Close()error{if s==nil||s.db==nil{return nil};return s.db.Close()}
