package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestNetworkIncidentRecordsPersistAcrossReopen(t *testing.T){path:=filepath.Join(t.TempDir(),"ircintel.db");store,err:=OpenSQLiteStore(path);if err!=nil{t.Fatal(err)};if err:=store.UpsertNetwork(Network{ID:"n1",Name:"Net"});err!=nil{t.Fatal(err)};if err:=store.UpsertNetworkServer(NetworkServer{ID:"s1",NetworkID:"n1",Name:"Server"});err!=nil{t.Fatal(err)};if err:=store.UpsertNetworkEndpoint(NetworkEndpoint{ID:"e1",ServerID:"s1",Host:"irc.example",Port:"6697",TLS:true});err!=nil{t.Fatal(err)};started:=time.Date(2026,9,9,10,0,0,0,time.UTC);recovered:=started.Add(5*time.Minute);duration:=int64(300);payload:=NetworkIncident{NetworkID:"n1",NetworkName:"Net",Status:"closed",Severity:"down",StartedAt:started,RecoveredAt:&recovered,DurationSeconds:&duration,AffectedEndpoints:1,TotalEndpoints:1};b,err:=json.Marshal(payload);if err!=nil{t.Fatal(err)};_,err=store.db.Exec(`INSERT INTO network_incident_records(network_id,started_at,status,severity,payload_json) VALUES(?,?,?,?,?)`,"n1",formatObservationTime(started),"closed","down",b);if err!=nil{t.Fatal(err)};if err:=store.Close();err!=nil{t.Fatal(err)};store,err=OpenSQLiteStore(path);if err!=nil{t.Fatal(err)};defer store.Close();items,err:=store.ListNetworkIncidentRecords(10);if err!=nil{t.Fatal(err)};if len(items)!=1||items[0].NetworkID!="n1"||items[0].Status!="closed"||items[0].Severity!="down"{t.Fatalf("unexpected records: %+v",items)}}
