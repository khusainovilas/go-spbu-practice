package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khusainovilas/go-spbu-practice/internal/domain"
)

const movieColumns = `id, title, year, director`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetLast(ctx context.Context) (domain.Movie, error) {
	var movie domain.Movie
	err := r.pool.QueryRow(ctx,
		`SELECT `+movieColumns+` FROM movies ORDER BY id DESC LIMIT 1`,
	).Scan(&movie.ID, &movie.Title, &movie.Year, &movie.Director)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Movie{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Movie{}, fmt.Errorf("get last movie: %w", err)
	}
	return movie, nil
}

func (r *Repository) ListRange(ctx context.Context, from, to int) ([]domain.Movie, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+movieColumns+` FROM movies WHERE id BETWEEN $1 AND $2 ORDER BY id`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	defer rows.Close()

	movies := make([]domain.Movie, 0)
	for rows.Next() {
		var movie domain.Movie
		if err := rows.Scan(&movie.ID, &movie.Title, &movie.Year, &movie.Director); err != nil {
			return nil, fmt.Errorf("scan movie: %w", err)
		}
		movies = append(movies, movie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	return movies, nil
}

func (r *Repository) ListByIDs(ctx context.Context, ids []int) ([]domain.Movie, error) {
	if len(ids) == 0 {
		return []domain.Movie{}, nil
	}

	values := make([]int64, len(ids))
	for i, id := range ids {
		values[i] = int64(id)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+movieColumns+` FROM movies WHERE id = ANY($1::bigint[]) ORDER BY id`,
		values,
	)
	if err != nil {
		return nil, fmt.Errorf("list movies by ids: %w", err)
	}
	defer rows.Close()

	movies := make([]domain.Movie, 0)
	for rows.Next() {
		var movie domain.Movie
		if err := rows.Scan(&movie.ID, &movie.Title, &movie.Year, &movie.Director); err != nil {
			return nil, fmt.Errorf("scan movie: %w", err)
		}
		movies = append(movies, movie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list movies by ids: %w", err)
	}
	return movies, nil
}

func (r *Repository) ListIDs(ctx context.Context) ([]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM movies ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list movie ids: %w", err)
	}
	defer rows.Close()

	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan movie id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list movie ids: %w", err)
	}
	return ids, nil
}

func (r *Repository) Upsert(ctx context.Context, movie domain.Movie) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO movies (`+movieColumns+`) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET
		     title = EXCLUDED.title,
		     year = EXCLUDED.year,
		     director = EXCLUDED.director`,
		movie.ID, movie.Title, movie.Year, movie.Director,
	)
	if err != nil {
		return fmt.Errorf("upsert movie %d: %w", movie.ID, err)
	}
	return nil
}
