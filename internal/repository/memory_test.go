package repository

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryInsertReportsConflicts(t *testing.T) {
	existing := Link{ShortCode: "aaaaaa", OriginalURL: "https://one.example", URLHash: []byte{1, 2, 3}}

	cases := []struct {
		name           string
		insert         Link
		wantConstraint string
	}{
		{
			name:           "reports conflict on duplicate short code",
			insert:         Link{ShortCode: "aaaaaa", OriginalURL: "https://two.example", URLHash: []byte{9, 9, 9}},
			wantConstraint: ConstraintShortCode,
		},
		{
			name:           "reports conflict on duplicate url hash",
			insert:         Link{ShortCode: "bbbbbb", OriginalURL: "https://one.example", URLHash: []byte{1, 2, 3}},
			wantConstraint: ConstraintURLHash,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := NewMemory()
			ctx := context.Background()

			if err := repo.Insert(ctx, existing); err != nil {
				t.Fatalf("insert existing: %v", err)
			}

			err := repo.Insert(ctx, c.insert)

			var conflict *ConflictError
			if !errors.As(err, &conflict) {
				t.Fatalf("error: got %v, want *ConflictError", err)
			}
			if conflict.Constraint != c.wantConstraint {
				t.Fatalf("constraint: got %q, want %q", conflict.Constraint, c.wantConstraint)
			}
		})
	}
}
