package store

import (
	"ai-travel/internal/domain"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *Store) GetTrip(ctx context.Context, owner, id string, version int64) (domain.Trip, error) {
	var b []byte
	e := s.Pool.QueryRow(ctx, "SELECT v.data FROM trips t JOIN trip_versions v ON v.trip_id=t.id AND v.version=CASE WHEN $3=0 THEN t.current_version ELSE $3 END WHERE t.owner_id=$1 AND t.id=$2", owner, id, version).Scan(&b)
	if e != nil {
		return domain.Trip{}, dbError(e)
	}
	t, e := decode[domain.Trip](b)
	return t, dbError(e)
}
func (s *Store) ListTrips(ctx context.Context, owner, id string, page, size int) ([]domain.Trip, bool, error) {
	query := `SELECT v.data #- '{plan,activities}' #- '{plan,routes}' FROM trips t JOIN trip_versions v ON v.trip_id=t.id AND v.version=t.current_version WHERE t.owner_id=$1 ORDER BY t.updated_at DESC,t.id LIMIT $2 OFFSET $3`
	args := []any{owner, size + 1, (page - 1) * size}
	if id != "" {
		query = `SELECT v.data #- '{plan,activities}' #- '{plan,routes}' FROM trips t JOIN trip_versions v ON v.trip_id=t.id WHERE t.owner_id=$1 AND t.id=$4 ORDER BY v.version DESC LIMIT $2 OFFSET $3`
		args = append(args, id)
		if _, e := s.GetTrip(ctx, owner, id, 0); e != nil {
			return nil, false, e
		}
	}
	rows, e := s.Pool.Query(ctx, query, args...)
	if e != nil {
		return nil, false, dbError(e)
	}
	defer rows.Close()
	out := []domain.Trip{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, false, dbError(e)
		}
		v, e := decode[domain.Trip](b)
		if e != nil {
			return nil, false, dbError(e)
		}
		v.Plan.Activities = []domain.Activity{}
		v.Plan.Routes = []domain.Route{}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		return nil, false, dbError(e)
	}
	more := len(out) > size
	if more {
		out = out[:size]
	}
	return out, more, nil
}
func (s *Store) Save(ctx context.Context, r Record, p domain.Plan) error {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return dbError(e)
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, "SELECT id FROM planning_jobs WHERE id=$1 AND attempt_id=$2 AND status='running' AND lease_expires_at>clock_timestamp() AND deadline_at>clock_timestamp() FOR UPDATE", r.Job.ID, r.AttemptID).Scan(&id)
	if e == pgx.ErrNoRows {
		return domain.Err(409, "LEASE_LOST", "任务租约已失效")
	}
	if e != nil {
		return dbError(e)
	}
	trip := domain.Trip{ID: domain.ID(), Version: 1, Constraints: r.Input.Constraints, Plan: p, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	if r.Base != nil {
		trip.ID = r.Base.ID
		trip.Version = r.Base.Version + 1
		tag, err := tx.Exec(ctx, "UPDATE trips SET current_version=$3,updated_at=now() WHERE id=$1 AND owner_id=$4 AND current_version=$2", trip.ID, r.Base.Version, trip.Version, r.OwnerID)
		if err != nil {
			return dbError(err)
		}
		if tag.RowsAffected() != 1 {
			_, e = tx.Exec(ctx, `UPDATE planning_jobs SET status='conflicted',stage='conflicted',error='{"code":"VERSION_CONFLICT","message":"行程已被修改，请读取最新版本","status":409}',updated_at=now() WHERE id=$1 AND attempt_id=$2 AND lease_expires_at>clock_timestamp() AND deadline_at>clock_timestamp()`, r.Job.ID, r.AttemptID)
			if e != nil {
				return dbError(e)
			}
			var status string
			if e = tx.QueryRow(ctx, "SELECT status FROM planning_jobs WHERE id=$1", r.Job.ID).Scan(&status); e != nil {
				return dbError(e)
			}
			if status != "conflicted" {
				return domain.Err(409, "LEASE_LOST", "任务租约已失效")
			}
			return dbError(tx.Commit(ctx))
		}
	} else {
		if _, e = tx.Exec(ctx, "INSERT INTO trips(id,owner_id,current_version) VALUES($1,$2,1)", trip.ID, r.OwnerID); e != nil {
			return dbError(e)
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO trip_versions(trip_id,version,source_job_id,data) VALUES($1,$2,$3,$4)", trip.ID, trip.Version, r.Job.ID, bytes(trip)); e != nil {
		return dbError(e)
	}
	for _, a := range p.Activities {
		if _, e = tx.Exec(ctx, "INSERT INTO trip_items(trip_id,version,item_id,data) VALUES($1,$2,$3,$4)", trip.ID, trip.Version, a.ID, bytes(a)); e != nil {
			return dbError(e)
		}
	}
	for _, route := range p.Routes {
		if _, e = tx.Exec(ctx, "INSERT INTO trip_routes(trip_id,version,from_item_id,to_item_id,data) VALUES($1,$2,$3,$4,$5)", trip.ID, trip.Version, route.FromItemID, route.ToItemID, bytes(route)); e != nil {
			return dbError(e)
		}
	}
	tag, e := tx.Exec(ctx, "UPDATE planning_jobs SET status='succeeded',stage='succeeded',trip_id=$2,result_version=$3,updated_at=now() WHERE id=$1 AND attempt_id=$4 AND status='running' AND lease_expires_at>clock_timestamp() AND deadline_at>clock_timestamp()", r.Job.ID, trip.ID, trip.Version, r.AttemptID)
	if e != nil {
		return dbError(e)
	}
	if tag.RowsAffected() != 1 {
		return domain.Err(409, "LEASE_LOST", "保存时任务已超时")
	}
	return dbError(tx.Commit(ctx))
}
