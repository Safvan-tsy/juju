// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"

	"github.com/juju/juju/domain"
	domainsequence "github.com/juju/juju/domain/sequence"
	"github.com/juju/juju/internal/errors"
)

type sequence struct {
	Namespace string `db:"namespace"`
	Value     uint64 `db:"value"`
}

// NextValue returns a monotonically incrementing int value for the given
// namespace.
// The first such value starts at 0.
func NextValue(ctx context.Context, preparer domain.Preparer, tx *sqlair.TX, namespace domainsequence.Namespace) (uint64, error) {
	seq := sequence{
		Namespace: namespace.String(),
	}
	updateStmt, err := preparer.Prepare(`
INSERT INTO sequence (namespace, value) VALUES ($sequence.namespace, 0)
ON CONFLICT DO UPDATE SET value = value + 1
`, seq)
	if err != nil {
		return 0, errors.Capture(err)
	}

	nextStmt, err := preparer.Prepare(`
SELECT &sequence.value FROM sequence WHERE namespace = $sequence.namespace
`, seq)
	if err != nil {
		return 0, errors.Capture(err)
	}

	// Increment the sequence number, we need to ensure that we affected the
	// sequence, otherwise we can end up with values that aren't unique.
	var outcome sqlair.Outcome
	err = tx.Query(ctx, updateStmt, seq).Get(&outcome)
	if err != nil {
		return 0, errors.Errorf("updating sequence number for namespace %q: %w", namespace, err)
	}
	if affected, err := outcome.Result().RowsAffected(); err != nil {
		return 0, errors.Errorf("getting affected rows for sequence number for namespace %q: %w", namespace, err)
	} else if affected != 1 {
		return 0, errors.Errorf("updating sequence number for namespace %q", namespace)
	}

	// Read the new value to return.
	err = tx.Query(ctx, nextStmt, seq).Get(&seq)
	if err != nil {
		return 0, errors.Errorf("reading sequence number for namespace %q: %w", namespace, err)
	}
	result := seq.Value
	return result, nil
}

type count struct {
	N uint64 `db:"n"`
}

// NextNValues returns the next n monotonically incrementing uint64 values for
// the given namespace. The values returned by this function will never be
// available for use again.
//
// If zero is supplied to this func no values will be created with no error
// returned.
func NextNValues(
	ctx context.Context,
	preparer domain.Preparer,
	tx *sqlair.TX,
	n uint64,
	namespace domainsequence.Namespace,
) ([]uint64, error) {
	if n == 0 {
		return nil, nil
	}

	seq := sequence{
		Namespace: namespace.String(),
		Value:     n - 1,
	}
	cnt := count{
		N: n,
	}

	updateStmt, err := preparer.Prepare(`
INSERT INTO sequence (*) VALUES ($sequence.*)
ON CONFLICT DO UPDATE SET value = value + $count.n
`, sequence{}, count{})
	if err != nil {
		return nil, errors.Capture(err)
	}

	latestValStmt, err := preparer.Prepare(`
SELECT &sequence.value FROM sequence WHERE namespace = $sequence.namespace
`, seq)
	if err != nil {
		return nil, errors.Capture(err)
	}

	// Increment the sequence number, we need to ensure that we affected the
	// sequence, otherwise we can end up with values that aren't unique.
	var outcome sqlair.Outcome
	err = tx.Query(ctx, updateStmt, seq, cnt).Get(&outcome)
	if err != nil {
		return nil, errors.Errorf("updating sequence number for namespace %q: %w", namespace, err)
	}
	if affected, err := outcome.Result().RowsAffected(); err != nil {
		return nil, errors.Errorf("getting affected rows for sequence number for namespace %q: %w", namespace, err)
	} else if affected != 1 {
		return nil, errors.Errorf("updating sequence number for namespace %q", namespace)
	}

	// Read the new value to return.
	err = tx.Query(ctx, latestValStmt, seq).Get(&seq)
	if err != nil {
		return nil, errors.Errorf("reading sequence number for namespace %q: %w", namespace, err)
	}

	// Be careful not to shoot ourselves in the foot.
	if seq.Value+1 < n {
		return nil, errors.Errorf("cannot reticulate splines")
	}
	start := seq.Value + 1
	start -= n

	rval := make([]uint64, 0, n)
	for i := range n {
		id := start + i
		rval = append(rval, id)
	}
	return rval, nil
}
