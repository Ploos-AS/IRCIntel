package core

import (
	"encoding/json"
	"errors"
)

type NetworkIncidentRecordReader interface {
	ListNetworkIncidentRecords(limit int) ([]NetworkIncident, error)
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

func (s *SQLiteStore) ListNetworkIncidentRecords(limit int) ([]NetworkIncident, error) {
	if s == nil || s.db == nil { return nil, errors.New("sqlite store is not open") }
	if limit < 1 || limit > 500 { return nil, errors.New("invalid network incident limit") }
	rows, err := s.db.Query(`SELECT payload_json FROM network_incident_records ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
	if err != nil { return nil, err }; defer rows.Close()
	out := make([]NetworkIncident,0)
	for rows.Next() { var payload []byte; if err:=rows.Scan(&payload); err!=nil{return nil,err}; var item NetworkIncident; if err:=json.Unmarshal(payload,&item);err!=nil{return nil,err}; out=append(out,item) }
	if err:=rows.Err();err!=nil{return nil,err}; return out,nil
}
