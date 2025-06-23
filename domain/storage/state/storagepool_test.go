// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"fmt"
	stdtesting "testing"

	"github.com/juju/tc"

	coredatabase "github.com/juju/juju/core/database"
	"github.com/juju/juju/domain"
	"github.com/juju/juju/domain/schema/testing"
	"github.com/juju/juju/domain/storage"
	domainstorage "github.com/juju/juju/domain/storage"
	storageerrors "github.com/juju/juju/domain/storage/errors"
	"github.com/juju/juju/internal/errors"
	internalstorage "github.com/juju/juju/internal/storage"
	dummystorage "github.com/juju/juju/internal/storage/provider/dummy"
)

type storagePoolStateSuite struct {
	testing.ModelSuite
}

func TestStoragePoolSuite(t *stdtesting.T) {
	tc.Run(t, &storagePoolStateSuite{})
}

func newStoragePoolState(factory coredatabase.TxnRunnerFactory) *State {
	return &State{
		StateBase: domain.NewStateBase(factory),
	}
}

func (s *storagePoolStateSuite) getStoragePoolOrigin(c *tc.C, name string) string {
	var origin string
	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT    spo.origin
FROM      storage_pool sp
LEFT JOIN storage_pool_origin spo ON spo.id = sp.origin_id
WHERE     sp.name = ?`, name).Scan(&origin)
	})
	c.Assert(err, tc.ErrorIsNil)
	return origin
}

func (s *storagePoolStateSuite) createStoragePoolWithOrigin(c *tc.C, sp domainstorage.StoragePool, origin string) {
	if sp.UUID == "" {
		spUUID, err := domainstorage.NewStoragePoolUUID()
		c.Assert(err, tc.ErrorIsNil)
		sp.UUID = spUUID.String()
	}

	err := s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var originID int
		err := tx.QueryRowContext(ctx, `
SELECT id
FROM   storage_pool_origin
WHERE  origin = ?`, origin).Scan(&originID)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
INSERT INTO storage_pool (uuid, name, type, origin_id)
VALUES (?, ?, ?, ?)`, sp.UUID, sp.Name, sp.Provider, originID)
		if err != nil {
			return err
		}
		if len(sp.Attrs) == 0 {
			return nil
		}

		for k, v := range sp.Attrs {
			_, err = tx.ExecContext(ctx, `
INSERT INTO storage_pool_attribute (storage_pool_uuid, key, value)
VALUES (?, ?, ?)`, sp.UUID, k, v)
			if err != nil {
				return err
			}
		}
		return nil
	})
	c.Assert(err, tc.ErrorIsNil)
}

func (s *storagePoolStateSuite) TestCreateStoragePool(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.GetStoragePoolByName(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	})
	origin := s.getStoragePoolOrigin(c, "ebs-fast")
	c.Assert(origin, tc.Equals, "user")
}

func (s *storagePoolStateSuite) TestCreateStoragePoolNoAttributes(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.GetStoragePoolByName(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
	})
	origin := s.getStoragePoolOrigin(c, "ebs-fast")
	c.Assert(origin, tc.Equals, "user")
}

func (s *storagePoolStateSuite) TestCreateStoragePoolAlreadyExists(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	err = st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIs, storageerrors.PoolAlreadyExists)
}

func (s *storagePoolStateSuite) TestUpdateCloudCredentialMissingName(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Provider: "ebs",
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(errors.Is(err, storageerrors.MissingPoolNameError), tc.IsTrue)
}

func (s *storagePoolStateSuite) TestUpdateCloudCredentialMissingProvider(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name: "ebs-fast",
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(errors.Is(err, storageerrors.MissingPoolTypeError), tc.IsTrue)
}

func (s *storagePoolStateSuite) TestReplaceStoragePool(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	sp2 := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"baz": "baz val",
		},
	}
	err = st.ReplaceStoragePool(ctx, sp2)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.GetStoragePoolByName(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"baz": "baz val",
		},
	})
}

func (s *storagePoolStateSuite) TestReplaceStoragePoolForbiddenForBuiltInPool(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	ctx := c.Context()
	s.createStoragePoolWithOrigin(c, domainstorage.StoragePool{
		Name:     "loop",
		Provider: "loop",
	}, "built-in")

	sp2 := domainstorage.StoragePool{
		Name:     "loop",
		Provider: "ebs",
	}
	err := st.ReplaceStoragePool(ctx, sp2)
	c.Assert(err, tc.ErrorMatches, `updating storage pool: built-in storage_pools are immutable, only insertions are allowed`)
}

// TestStoragePoolImmutableOrigin tests that the origin of a storage pool cannot be changed
// after it has been created. This is not a state method test but a schema trigger test.
func (s *storagePoolStateSuite) TestStoragePoolImmutableOrigin(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)
	origin := s.getStoragePoolOrigin(c, "ebs-fast")
	c.Assert(origin, tc.Equals, "user")

	err = s.TxnRunner().StdTxn(c.Context(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
UPDATE storage_pool
SET    origin_id = (SELECT id FROM storage_pool_origin WHERE origin = 'built-in')
WHERE  name = ?`, "ebs-fast")
		return err
	})
	c.Assert(err, tc.ErrorMatches, `storage pool origin cannot be changed`)
}

func (s *storagePoolStateSuite) TestReplaceStoragePoolNoAttributes(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	sp2 := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
	}
	err = st.ReplaceStoragePool(ctx, sp2)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.GetStoragePoolByName(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
	})
}

func (s *storagePoolStateSuite) TestReplaceStoragePoolNotFound(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"baz": "baz val",
		},
	}
	ctx := c.Context()
	err := st.ReplaceStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIs, storageerrors.PoolNotFoundError)
}

func (s *storagePoolStateSuite) TestDeleteStoragePool(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	err = st.DeleteStoragePool(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIsNil)

	_, err = st.GetStoragePoolByName(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIs, storageerrors.PoolNotFoundError)
}

func (s *storagePoolStateSuite) TestDeleteStoragePoolNotFound(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	ctx := c.Context()
	err := st.DeleteStoragePool(ctx, "ebs-fast")
	c.Assert(err, tc.ErrorIs, storageerrors.PoolNotFoundError)
}

func (s *storagePoolStateSuite) TestDeleteStoragePoolFailedForBuiltInPool(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	ctx := c.Context()
	s.createStoragePoolWithOrigin(c, domainstorage.StoragePool{
		Name:     "loop",
		Provider: "loop",
	}, "built-in")

	err := st.DeleteStoragePool(ctx, "loop")
	c.Assert(err, tc.ErrorMatches, `built-in storage_pools are immutable, only insertions are allowed`)
}

// ensureBuiltInStoragePools ensures that the built-in storage pools are created in the state.
// This is a temporary workaround until we implement the built-in and default storage pools
// insertion during model creation.
func (s *storagePoolStateSuite) ensureBuiltInStoragePools(c *tc.C) []domainstorage.StoragePool {
	pools, err := domainstorage.BuiltInStoragePools()
	c.Assert(err, tc.ErrorIsNil)

	for _, sp := range pools {
		s.createStoragePoolWithOrigin(c, sp, "built-in")
	}
	c.Logf("Created built-in storage pools: %#v", pools)
	return pools
}

// ensureProviderDefaultStoragePools ensures that the default storage pools are created in the state.
// This is a temporary workaround until we implement the default storage pools insertion during model creation.
func (s *storagePoolStateSuite) ensureProviderDefaultStoragePools(c *tc.C) []domainstorage.StoragePool {
	p1, err := internalstorage.NewConfig("pool1", "whatever", map[string]any{"1": "2"})
	c.Assert(err, tc.ErrorIsNil)
	p2, err := internalstorage.NewConfig("pool2", "whatever", map[string]any{
		"3": "4",
		"5": "6",
	})
	c.Assert(err, tc.ErrorIsNil)
	provider := &dummystorage.StorageProvider{
		DefaultPools_: []*internalstorage.Config{p1, p2},
	}

	registry := internalstorage.StaticProviderRegistry{
		Providers: map[internalstorage.ProviderType]internalstorage.Provider{
			"whatever": provider,
		},
	}

	poolCfgs, err := storage.DefaultStoragePools(registry)
	c.Assert(err, tc.ErrorIsNil)

	var pools []domainstorage.StoragePool
	for _, pcfg := range poolCfgs {
		sp := domainstorage.StoragePool{
			Name:     pcfg.Name(),
			Provider: string(pcfg.Provider()),
			Attrs:    make(map[string]string),
		}
		for k, v := range pcfg.Attrs() {
			sp.Attrs[k] = fmt.Sprintf("%s", v)
		}
		s.createStoragePoolWithOrigin(c, sp, "provider-default")

		pools = append(pools, sp)
	}
	c.Logf("Created provider default storage pools: %#v", pools)
	return pools
}

func (s *storagePoolStateSuite) TestListStoragePoolsWithoutBuiltIns(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	defaultPools := s.ensureProviderDefaultStoragePools(c)

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	sp2 := domainstorage.StoragePool{
		Name:     "ebs-faster",
		Provider: "ebs",
		Attrs: map[string]string{
			"baz": "baz val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)
	err = st.CreateStoragePool(ctx, sp2)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.ListStoragePoolsWithoutBuiltins(c.Context())
	c.Assert(err, tc.ErrorIsNil)

	expected := []domainstorage.StoragePool{sp, sp2}
	expected = append(expected, defaultPools...)
	c.Assert(out, tc.SameContents, expected)
}

func (s *storagePoolStateSuite) TestListStoragePoolsWithoutBuiltinsEmpty(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	out, err := st.ListStoragePoolsWithoutBuiltins(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.HasLen, 0)
}

func (s *storagePoolStateSuite) TestListStoragePools(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	builtInPools := s.ensureBuiltInStoragePools(c)
	defaultPools := s.ensureProviderDefaultStoragePools(c)

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}
	sp2 := domainstorage.StoragePool{
		Name:     "ebs-faster",
		Provider: "ebs",
		Attrs: map[string]string{
			"baz": "baz val",
		},
	}
	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)
	err = st.CreateStoragePool(ctx, sp2)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.ListStoragePools(c.Context())
	c.Assert(err, tc.ErrorIsNil)

	expected := []domainstorage.StoragePool{sp, sp2}
	expected = append(expected, builtInPools...)
	expected = append(expected, defaultPools...)
	c.Assert(out, tc.SameContents, expected)
}

func (s *storagePoolStateSuite) TestListStoragePoolsNoUserPools(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	builtInPools := s.ensureBuiltInStoragePools(c)
	defaultPools := s.ensureProviderDefaultStoragePools(c)

	out, err := st.ListStoragePools(c.Context())
	c.Assert(err, tc.ErrorIsNil)

	var expected []domainstorage.StoragePool
	expected = append(expected, builtInPools...)
	expected = append(expected, defaultPools...)
	c.Assert(out, tc.SameContents, expected)
}

func (s *storagePoolStateSuite) TestListStoragePoolsByNamesAndProviders(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}

	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.ListStoragePoolsByNamesAndProviders(c.Context(),
		domainstorage.Names{"pool1", "pool2", "ebs-fast", "ebs-fast", "loop", ""},
		domainstorage.Providers{"whatever", "ebs", "ebs", "loop", ""},
	)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.SameContents, []domainstorage.StoragePool{
		sp,
		{
			Name:     "loop",
			Provider: "loop",
		},
		{
			Name:     "pool1",
			Provider: "whatever",
			Attrs: map[string]string{
				"1": "2",
			},
		},
		{
			Name:     "pool2",
			Provider: "whatever",
			Attrs: map[string]string{
				"3": "4",
				"5": "6",
			},
		},
	})
}

func (s *storagePoolStateSuite) TestListStoragePoolsByNames(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}

	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.ListStoragePoolsByNames(c.Context(), domainstorage.Names{"pool1", "ebs-fast", "loop", "loop", ""})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.SameContents, []domainstorage.StoragePool{
		sp,
		{
			Name:     "loop",
			Provider: "loop",
		},
		{
			Name:     "pool1",
			Provider: "whatever",
			Attrs: map[string]string{
				"1": "2",
			},
		},
	})
}

func (s *storagePoolStateSuite) TestListStoragePoolsByProviders(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}

	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.ListStoragePoolsByProviders(c.Context(), domainstorage.Providers{"whatever", "ebs", "loop", "loop", ""})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.SameContents, []domainstorage.StoragePool{
		sp,
		{
			Name:     "loop",
			Provider: "loop",
		},
		{
			Name:     "pool1",
			Provider: "whatever",
			Attrs: map[string]string{
				"1": "2",
			},
		},
		{
			Name:     "pool2",
			Provider: "whatever",
			Attrs: map[string]string{
				"3": "4",
				"5": "6",
			},
		},
	})
}

func (s *storagePoolStateSuite) TestGetStoragePoolByName(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	sp := domainstorage.StoragePool{
		Name:     "ebs-fast",
		Provider: "ebs",
		Attrs: map[string]string{
			"foo": "foo val",
			"bar": "bar val",
		},
	}

	ctx := c.Context()
	err := st.CreateStoragePool(ctx, sp)
	c.Assert(err, tc.ErrorIsNil)

	out, err := st.GetStoragePoolByName(c.Context(), "ebs-fast")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, sp)
}

func (s *storagePoolStateSuite) TestGetStoragePoolByNameBuiltIn(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	out, err := st.GetStoragePoolByName(c.Context(), "loop")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, domainstorage.StoragePool{
		Name:     "loop",
		Provider: "loop",
	})
}

func (s *storagePoolStateSuite) TestGetStoragePoolByNameDefault(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	out, err := st.GetStoragePoolByName(c.Context(), "pool1")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(out, tc.DeepEquals, domainstorage.StoragePool{
		Name:     "pool1",
		Provider: "whatever",
		Attrs: map[string]string{
			"1": "2",
		},
	})
}

func (s *storagePoolStateSuite) TestGetStoragePoolByNameNotFound(c *tc.C) {
	st := newStoragePoolState(s.TxnRunnerFactory())

	_ = s.ensureBuiltInStoragePools(c)
	_ = s.ensureProviderDefaultStoragePools(c)

	_, err := st.GetStoragePoolByName(c.Context(), "non-existent")
	c.Assert(err, tc.ErrorIs, storageerrors.PoolNotFoundError)
}
