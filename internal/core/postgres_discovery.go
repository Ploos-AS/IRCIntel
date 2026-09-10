package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func migratePostgresDiscovery(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS discovery_candidates (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port TEXT NOT NULL,
    tls BOOLEAN NOT NULL,
    source TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('pending','accepted','rejected')),
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    seen_count BIGINT NOT NULL CHECK(seen_count > 0)
);
CREATE INDEX IF NOT EXISTS idx_pg_discovery_candidates_status_seen
    ON discovery_candidates(status, last_seen DESC, id);
CREATE TABLE IF NOT EXISTS discovery_reviews (
    id BIGSERIAL PRIMARY KEY,
    candidate_id TEXT NOT NULL REFERENCES discovery_candidates(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK(status IN ('accepted','rejected')),
    reviewer TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pg_discovery_reviews_time
    ON discovery_reviews(reviewed_at DESC, candidate_id);
CREATE TABLE IF NOT EXISTS discovery_promotions (
    id BIGSERIAL PRIMARY KEY,
    candidate_id TEXT NOT NULL UNIQUE REFERENCES discovery_candidates(id) ON DELETE RESTRICT,
    endpoint_id TEXT NOT NULL REFERENCES network_endpoints(id) ON DELETE RESTRICT,
    server_id TEXT NOT NULL REFERENCES network_servers(id) ON DELETE RESTRICT,
    promoter TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    promoted_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pg_discovery_promotions_time
    ON discovery_promotions(promoted_at DESC, candidate_id);
`)
	return err
}

func (s *PostgresStore) UpsertDiscoveryCandidate(input DiscoveryCandidateInput, observedAt time.Time) (DiscoveryCandidate, error) {
	if s == nil || s.pool == nil { return DiscoveryCandidate{}, errors.New("postgres store is not open") }
	input.Host = strings.ToLower(strings.TrimSpace(input.Host))
	input.Port = strings.TrimSpace(input.Port)
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	if input.Host == "" || input.Port == "" || input.Source == "" { return DiscoveryCandidate{}, errors.New("host, port, and source are required") }
	if observedAt.IsZero() { observedAt = time.Now().UTC() }
	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout); defer cancel()
	id := discoveryCandidateID(input)
	var item DiscoveryCandidate
	err := s.pool.QueryRow(ctx, `
INSERT INTO discovery_candidates (id, host, port, tls, source, source_ref, status, first_seen, last_seen, seen_count)
VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$7,1)
ON CONFLICT(id) DO UPDATE SET source_ref=excluded.source_ref,last_seen=excluded.last_seen,seen_count=discovery_candidates.seen_count+1
RETURNING id,host,port,tls,source,source_ref,status,first_seen,last_seen,seen_count`,
		id,input.Host,input.Port,input.TLS,input.Source,input.SourceRef,observedAt.UTC()).Scan(
		&item.ID,&item.Host,&item.Port,&item.TLS,&item.Source,&item.SourceRef,&item.Status,&item.FirstSeen,&item.LastSeen,&item.SeenCount)
	item.FirstSeen=item.FirstSeen.UTC(); item.LastSeen=item.LastSeen.UTC()
	return item, err
}

func (s *PostgresStore) ListDiscoveryCandidates(status string) ([]DiscoveryCandidate, error) {
	if s == nil || s.pool == nil { return nil, errors.New("postgres store is not open") }
	status = strings.ToLower(strings.TrimSpace(status))
	if status!="" && status!="pending" && status!="accepted" && status!="rejected" { return nil, errors.New("invalid discovery status") }
	ctx,cancel:=context.WithTimeout(context.Background(),postgresOperationTimeout); defer cancel()
	statement:=`SELECT id,host,port,tls,source,source_ref,status,first_seen,last_seen,seen_count FROM discovery_candidates`
	args:=[]any{}
	if status!="" { statement+=` WHERE status=$1`; args=append(args,status) }
	statement+=` ORDER BY last_seen DESC,id`
	rows,err:=s.pool.Query(ctx,statement,args...); if err!=nil{return nil,err}; defer rows.Close()
	out:=make([]DiscoveryCandidate,0)
	for rows.Next(){var item DiscoveryCandidate;if err:=rows.Scan(&item.ID,&item.Host,&item.Port,&item.TLS,&item.Source,&item.SourceRef,&item.Status,&item.FirstSeen,&item.LastSeen,&item.SeenCount);err!=nil{return nil,err};item.FirstSeen=item.FirstSeen.UTC();item.LastSeen=item.LastSeen.UTC();out=append(out,item)}
	return out,rows.Err()
}

func (s *PostgresStore) ReviewDiscoveryCandidate(input DiscoveryReviewInput, reviewedAt time.Time) (DiscoveryReview, error) {
	if s == nil || s.pool == nil { return DiscoveryReview{}, errors.New("postgres store is not open") }
	input.CandidateID=strings.TrimSpace(input.CandidateID); input.Status=strings.ToLower(strings.TrimSpace(input.Status)); input.Reviewer=strings.TrimSpace(input.Reviewer); input.Note=strings.TrimSpace(input.Note)
	if input.CandidateID==""||input.Reviewer==""||(input.Status!="accepted"&&input.Status!="rejected"){return DiscoveryReview{},errors.New("invalid discovery review")}
	if reviewedAt.IsZero(){reviewedAt=time.Now().UTC()}
	ctx,cancel:=context.WithTimeout(context.Background(),postgresOperationTimeout);defer cancel()
	tx,err:=s.pool.Begin(ctx);if err!=nil{return DiscoveryReview{},err};defer tx.Rollback(ctx)
	var current string
	err=tx.QueryRow(ctx,`SELECT status FROM discovery_candidates WHERE id=$1 FOR UPDATE`,input.CandidateID).Scan(&current)
	if errors.Is(err,pgx.ErrNoRows){return DiscoveryReview{},ErrDiscoveryCandidateNotFound};if err!=nil{return DiscoveryReview{},err}
	if current!="pending"{return DiscoveryReview{},ErrDiscoveryCandidateAlreadyReviewed}
	if _,err=tx.Exec(ctx,`INSERT INTO discovery_reviews(candidate_id,status,reviewer,note,reviewed_at) VALUES($1,$2,$3,$4,$5)`,input.CandidateID,input.Status,input.Reviewer,input.Note,reviewedAt.UTC());err!=nil{return DiscoveryReview{},err}
	if _,err=tx.Exec(ctx,`UPDATE discovery_candidates SET status=$1 WHERE id=$2`,input.Status,input.CandidateID);err!=nil{return DiscoveryReview{},err}
	if err=tx.Commit(ctx);err!=nil{return DiscoveryReview{},err}
	return DiscoveryReview{CandidateID:input.CandidateID,Status:input.Status,Reviewer:input.Reviewer,Note:input.Note,ReviewedAt:reviewedAt.UTC()},nil
}

func (s *PostgresStore) ListDiscoveryReviews() ([]DiscoveryReview,error){
	if s==nil||s.pool==nil{return nil,errors.New("postgres store is not open")}
	ctx,cancel:=context.WithTimeout(context.Background(),postgresOperationTimeout);defer cancel()
	rows,err:=s.pool.Query(ctx,`SELECT candidate_id,status,reviewer,note,reviewed_at FROM discovery_reviews ORDER BY reviewed_at DESC,candidate_id`);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]DiscoveryReview,0);for rows.Next(){var item DiscoveryReview;if err:=rows.Scan(&item.CandidateID,&item.Status,&item.Reviewer,&item.Note,&item.ReviewedAt);err!=nil{return nil,err};item.ReviewedAt=item.ReviewedAt.UTC();out=append(out,item)};return out,rows.Err()
}

func (s *PostgresStore) PromoteDiscoveryCandidate(input DiscoveryPromotionInput, promotedAt time.Time) (DiscoveryPromotion,error){
	if s==nil||s.pool==nil{return DiscoveryPromotion{},errors.New("postgres store is not open")}
	input.CandidateID=strings.TrimSpace(input.CandidateID);input.EndpointID=strings.TrimSpace(input.EndpointID);input.ServerID=strings.TrimSpace(input.ServerID);input.Promoter=strings.TrimSpace(input.Promoter);input.Note=strings.TrimSpace(input.Note)
	if input.CandidateID==""||input.EndpointID==""||input.ServerID==""||input.Promoter==""{return DiscoveryPromotion{},errors.New("invalid discovery promotion")}
	if promotedAt.IsZero(){promotedAt=time.Now().UTC()}
	ctx,cancel:=context.WithTimeout(context.Background(),postgresOperationTimeout);defer cancel();tx,err:=s.pool.Begin(ctx);if err!=nil{return DiscoveryPromotion{},err};defer tx.Rollback(ctx)
	var host,port,status string;var tls bool
	err=tx.QueryRow(ctx,`SELECT host,port,tls,status FROM discovery_candidates WHERE id=$1 FOR UPDATE`,input.CandidateID).Scan(&host,&port,&tls,&status)
	if errors.Is(err,pgx.ErrNoRows){return DiscoveryPromotion{},ErrDiscoveryCandidateNotFound};if err!=nil{return DiscoveryPromotion{},err};if status!="accepted"{return DiscoveryPromotion{},ErrDiscoveryCandidateNotAccepted}
	var exists bool
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM discovery_promotions WHERE candidate_id=$1)`,input.CandidateID).Scan(&exists);err!=nil{return DiscoveryPromotion{},err};if exists{return DiscoveryPromotion{},ErrDiscoveryCandidatePromoted}
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM network_servers WHERE id=$1)`,input.ServerID).Scan(&exists);err!=nil{return DiscoveryPromotion{},err};if !exists{return DiscoveryPromotion{},ErrDiscoveryPromotionServer}
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM network_endpoints WHERE id=$1 OR (server_id=$2 AND host=$3 AND port=$4 AND tls=$5))`,input.EndpointID,input.ServerID,host,port,tls).Scan(&exists);err!=nil{return DiscoveryPromotion{},err};if exists{return DiscoveryPromotion{},ErrDiscoveryPromotionEndpointConflict}
	if _,err=tx.Exec(ctx,`INSERT INTO network_endpoints(id,server_id,host,port,tls) VALUES($1,$2,$3,$4,$5)`,input.EndpointID,input.ServerID,host,port,tls);err!=nil{return DiscoveryPromotion{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO discovery_promotions(candidate_id,endpoint_id,server_id,promoter,note,promoted_at) VALUES($1,$2,$3,$4,$5,$6)`,input.CandidateID,input.EndpointID,input.ServerID,input.Promoter,input.Note,promotedAt.UTC());err!=nil{return DiscoveryPromotion{},err}
	if err=tx.Commit(ctx);err!=nil{return DiscoveryPromotion{},err}
	return DiscoveryPromotion{CandidateID:input.CandidateID,EndpointID:input.EndpointID,ServerID:input.ServerID,Promoter:input.Promoter,Note:input.Note,PromotedAt:promotedAt.UTC()},nil
}

func (s *PostgresStore) ListDiscoveryPromotions()([]DiscoveryPromotion,error){
	if s==nil||s.pool==nil{return nil,errors.New("postgres store is not open")};ctx,cancel:=context.WithTimeout(context.Background(),postgresOperationTimeout);defer cancel()
	rows,err:=s.pool.Query(ctx,`SELECT candidate_id,endpoint_id,server_id,promoter,note,promoted_at FROM discovery_promotions ORDER BY promoted_at DESC,candidate_id`);if err!=nil{return nil,err};defer rows.Close();out:=make([]DiscoveryPromotion,0)
	for rows.Next(){var item DiscoveryPromotion;if err:=rows.Scan(&item.CandidateID,&item.EndpointID,&item.ServerID,&item.Promoter,&item.Note,&item.PromotedAt);err!=nil{return nil,err};item.PromotedAt=item.PromotedAt.UTC();out=append(out,item)};return out,rows.Err()
}
