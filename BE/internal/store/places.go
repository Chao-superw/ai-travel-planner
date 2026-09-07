package store

import (
	"ai-travel/internal/domain"
	"context"
	"github.com/jackc/pgx/v5"
)

func (s *Store) RememberPlaces(ctx context.Context, places []domain.Place) ([]domain.Place, error) {
	out := []domain.Place{}
	for _, p := range places {
		p.Version = 1
		var b []byte
		var ver int64
		e := s.Pool.QueryRow(ctx, `INSERT INTO places(id,provider,provider_poi_id,data) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET data=CASE WHEN places.curated THEN places.data ELSE EXCLUDED.data END RETURNING data,version`, p.ID, p.Provider, p.ProviderPOIID, bytes(p)).Scan(&b, &ver)
		if e != nil {
			return nil, dbError(e)
		}
		p, e = decode[domain.Place](b)
		if e != nil {
			return nil, dbError(e)
		}
		p.Version = ver
		if p.Active {
			out = append(out, p)
		}
	}
	return out, nil
}
func (s *Store) Place(ctx context.Context, id string) (domain.Place, error) {
	var b []byte
	var ver int64
	e := s.Pool.QueryRow(ctx, "SELECT data,version FROM places WHERE id=$1", id).Scan(&b, &ver)
	if e != nil {
		return domain.Place{}, dbError(e)
	}
	p, e := decode[domain.Place](b)
	p.Version = ver
	return p, dbError(e)
}
func (s *Store) Curate(ctx context.Context, p domain.Place, create bool) (domain.Place, error) {
	var e error
	if create {
		var b []byte
		var ver int64
		e = s.Pool.QueryRow(ctx, "UPDATE places SET curated=true,version=version+1,data=$2 WHERE id=$1 AND NOT curated RETURNING data,version", p.ID, bytes(p)).Scan(&b, &ver)
		if e == pgx.ErrNoRows {
			return p, domain.Err(409, "CONFLICT", "该景点已经维护，请使用更新接口")
		}
		p.Version = ver
	} else {
		p.Version++
		tag, err := s.Pool.Exec(ctx, "UPDATE places SET curated=true,version=$3,data=$2 WHERE id=$1 AND version=$4", p.ID, bytes(p), p.Version, p.Version-1)
		e = err
		if e == nil && tag.RowsAffected() != 1 {
			return p, domain.Err(409, "VERSION_CONFLICT", "景点资料版本已改变")
		}
	}
	return p, dbError(e)
}
