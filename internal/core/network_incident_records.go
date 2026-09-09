package core

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type NetworkIncidentRecordQuery struct {
	Status    string
	Severity  string
	NetworkID string
	Since     time.Time
	Until     time.Time
	Limit     int
}

type NetworkIncidentRecordReader interface {
	ListNetworkIncidentRecords(query NetworkIncidentRecordQuery) ([]NetworkIncident, error)
}

func (s *SQLiteStore) refreshNetworkIncidentRecords() error {
	if s == nil || s.db == nil { return errors.New("sqlite store is not open") }
	snapshot, err := s.RegistrySnapshot(); if err != nil { return err }
	endpoints, err := s.ListIncidentRecords(IncidentRecordQuery{Limit: 500}); if err != nil { return err }
	items := deriveNetworkIncidents(snapshot, endpoints)
	if len(items) == 0 { return nil }
	tx, err := s.db.Begin(); if err != nil { return err }; defer tx.Rollback()
	for _, item := range items {
		payload, err := json.Marshal(item); if err != nil { return err }
		_, err = tx.Exec(`INSERT INTO network_incident_records
(network_id, started_at, status, severity, payload_json) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(network_id, started_at) DO UPDATE SET
status=excluded.status, severity=excluded.severity, payload_json=excluded.payload_json`,
			item.NetworkID, formatObservationTime(item.StartedAt), item.Status, item.Severity, payload)
		if err != nil { return err }
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListNetworkIncidentRecords(query NetworkIncidentRecordQuery) ([]NetworkIncident, error) {
	if s == nil || s.db == nil { return nil, errors.New("sqlite store is not open") }
	if query.Limit < 1 || query.Limit > 500 { return nil, errors.New("invalid network incident limit") }
	if query.Status != "" && query.Status != "open" && query.Status != "closed" { return nil, errors.New("invalid network incident status") }
	if query.Severity != "" && query.Severity != "degraded" && query.Severity != "down" { return nil, errors.New("invalid network incident severity") }
	if !query.Since.IsZero() && !query.Until.IsZero() && query.Since.After(query.Until) { return nil, errors.New("invalid network incident time range") }
	where := make([]string, 0, 5); args := make([]any, 0, 6)
	if query.Status != "" { where=append(where,"status = ?"); args=append(args,query.Status) }
	if query.Severity != "" { where=append(where,"severity = ?"); args=append(args,query.Severity) }
	if query.NetworkID != "" { where=append(where,"network_id = ?"); args=append(args,query.NetworkID) }
	if !query.Since.IsZero() { where=append(where,"started_at >= ?"); args=append(args,formatObservationTime(query.Since)) }
	if !query.Until.IsZero() { where=append(where,"started_at <= ?"); args=append(args,formatObservationTime(query.Until)) }
	statement := "SELECT payload_json FROM network_incident_records"
	if len(where)>0 { statement += " WHERE " + strings.Join(where," AND ") }
	statement += " ORDER BY started_at DESC, id DESC LIMIT ?"; args=append(args,query.Limit)
	rows, err := s.db.Query(statement,args...); if err != nil { return nil,err }; defer rows.Close()
	out:=make([]NetworkIncident,0); for rows.Next(){var payload []byte;if err:=rows.Scan(&payload);err!=nil{return nil,err};var item NetworkIncident;if err:=json.Unmarshal(payload,&item);err!=nil{return nil,err};out=append(out,item)}
	if err:=rows.Err();err!=nil{return nil,err};return out,nil
}
